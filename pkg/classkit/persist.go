package classkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by DeleteDef when no class file matches the id.
var ErrNotFound = errors.New("class not found")

// ValidationError marks a definition that failed classkit validation. Handlers
// use it to answer 400 instead of 500 for bad user input.
type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// IsValidationError reports whether err came from ValidateDef/SaveDef.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

// ActiveDir is the directory used by the last successful LoadDir/LoadWorld.
// When no pack has loaded it falls back to the default world-pack path.
func ActiveDir() string {
	mu.RLock()
	dir := activeDir
	mu.RUnlock()
	if strings.TrimSpace(dir) != "" {
		return dir
	}
	return filepath.Join("data", "classes")
}

func setActiveDir(dir string) {
	mu.Lock()
	activeDir = dir
	mu.Unlock()
}

// ValidateDef is the exported form of the load-time validator. The path is a
// stable label so callers get the same error text as a pack load would emit.
func ValidateDef(d *Def) error {
	if d == nil {
		return &ValidationError{Err: errors.New("class is required")}
	}
	if err := validate(d, "class"); err != nil {
		return &ValidationError{Err: err}
	}
	return nil
}

// SaveDef validates d, writes it atomically to <dir>/<id>.yaml, and reloads the
// catalog from dir. An empty dir uses ActiveDir. If the reload fails the file
// is rolled back so a rejected save does not leave a broken pack behind.
func SaveDef(dir string, d *Def) error {
	if d == nil {
		return &ValidationError{Err: errors.New("class is required")}
	}
	if strings.TrimSpace(dir) == "" {
		dir = ActiveDir()
	}
	if err := ValidateDef(d); err != nil {
		return err
	}

	id := norm(d.ID)
	if !safeFileID(id) {
		return &ValidationError{Err: fmt.Errorf("class id %q is not a safe file name", d.ID)}
	}
	d.ID = id

	data, err := yaml.Marshal(d)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	target := filepath.Join(dir, id+".yaml")
	prev, readErr := os.ReadFile(target)
	existed := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}

	tmp, err := os.CreateTemp(dir, "."+id+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTmp := true
	defer func() {
		if keepTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	keepTmp = false

	if err := reload(dir); err != nil {
		if existed {
			_ = os.WriteFile(target, prev, 0o644)
		} else {
			_ = os.Remove(target)
		}
		_ = reload(dir)
		return err
	}
	return nil
}

// DeleteDef removes the named class file and reloads the catalog. The id is
// matched against the YAML `id` field (not the file name) after normalisation.
func DeleteDef(dir, id string) error {
	if strings.TrimSpace(dir) == "" {
		dir = ActiveDir()
	}
	want := norm(id)
	if want == "" {
		return &ValidationError{Err: errors.New("class id is required")}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return err
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var d Def
		if err := yaml.Unmarshal(raw, &d); err != nil {
			continue
		}
		if norm(d.ID) != want {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return reload(dir)
}

// FindDef resolves an id, alias, template id, or display name to its class.
func FindDef(id string) *Def {
	return Lookup(id)
}

// reload re-reads dir and keeps the previous catalog when the directory is
// missing or unreadable. An existing empty directory clears the catalog so
// deleting the last class does not leave the old row visible.
func reload(dir string) error {
	n, err := LoadDir(dir)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	st, statErr := os.Stat(dir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return nil
		}
		return statErr
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	setActiveDir(dir)
	install(nil, true)
	return nil
}

func safeFileID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, "/\\\x00")
}
