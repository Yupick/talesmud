package llmprompt

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var embeddedPrompts embed.FS

// Registry resolves, lists and renders prompt templates.
type Registry interface {
	Get(id string) (*Template, error)
	List() []*Template
	Render(id string, vars map[string]any) (Rendered, error)
}

// Savable is the optional override-write capability. It is only available
// when the registry was created with a non-empty override directory.
type Savable interface {
	Save(t *Template) error
}

type registry struct {
	dir     string
	byID    map[string]*Template
	order   []string
}

// NewRegistry loads the embedded templates and, when dir is non-empty,
// overlays any YAML file found there (dev overrides). It fails when an
// embedded template is malformed or when a mandatory prompt is missing.
func NewRegistry(dir string) (Registry, error) {
	r := &registry{dir: strings.TrimSpace(dir), byID: map[string]*Template{}}
	if err := r.loadEmbedded(); err != nil {
		return nil, err
	}
	if r.dir != "" {
		if err := r.loadDir(); err != nil {
			return nil, err
		}
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	r.rebuildOrder()
	return r, nil
}

func (r *registry) loadEmbedded() error {
	entries, err := embeddedPrompts.ReadDir("prompts")
	if err != nil {
		return fmt.Errorf("llmprompt: read embedded prompts: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := embeddedPrompts.ReadFile("prompts/" + e.Name())
		if err != nil {
			return fmt.Errorf("llmprompt: read %s: %w", e.Name(), err)
		}
		t, err := parseTemplate(raw)
		if err != nil {
			return fmt.Errorf("llmprompt: embedded %s: %w", e.Name(), err)
		}
		r.byID[t.ID] = t
	}
	return nil
}

func (r *registry) loadDir() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("llmprompt: read %s: %w", r.dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return fmt.Errorf("llmprompt: read %s: %w", e.Name(), err)
		}
		t, err := parseTemplate(raw)
		if err != nil {
			return fmt.Errorf("llmprompt: override %s: %w", e.Name(), err)
		}
		r.byID[t.ID] = t
	}
	return nil
}

func (r *registry) validate() error {
	for _, id := range mandatoryPrompts {
		if _, ok := r.byID[id]; !ok {
			return fmt.Errorf("llmprompt: mandatory template %q is missing", id)
		}
	}
	return nil
}

// mandatoryPrompts are the templates the Corte A consumers require.
var mandatoryPrompts = []string{
	"character.name",
	"character.description",
	"quest.generate",
	"npc.dialogue",
	"room.description",
	"dm.narration",
}

func (r *registry) rebuildOrder() {
	r.order = r.order[:0]
	for id := range r.byID {
		r.order = append(r.order, id)
	}
	sort.Strings(r.order)
}

func (r *registry) Get(id string) (*Template, error) {
	t, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("llmprompt: unknown template %q", id)
	}
	return t, nil
}

func (r *registry) List() []*Template {
	out := make([]*Template, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// Save writes a template override as YAML into the override directory. It
// reports an error when overrides are not enabled.
func (r *registry) Save(t *Template) error {
	if r.dir == "" {
		return fmt.Errorf("llmprompt: prompt overrides are not enabled (set LLM_PROMPT_DIR)")
	}
	if t == nil || strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: template id is required", ErrBadVars)
	}
	if err := validateTemplate(t); err != nil {
		return err
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("llmprompt: create override dir: %w", err)
	}
	raw, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("llmprompt: marshal template: %w", err)
	}
	path := filepath.Join(r.dir, sanitizeIdentifier(t.ID)+".yaml")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("llmprompt: write override: %w", err)
	}
	r.byID[t.ID] = t
	r.rebuildOrder()
	return nil
}

func parseTemplate(raw []byte) (*Template, error) {
	var t Template
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&t); err != nil {
		return nil, err
	}
	if err := validateTemplate(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func validateTemplate(t *Template) error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: template id is required", ErrBadVars)
	}
	if !outputModeValid(t.Output) {
		return fmt.Errorf("%w: template %q has invalid output %q", ErrBadVars, t.ID, t.Output)
	}
	if strings.TrimSpace(t.System) == "" {
		return fmt.Errorf("%w: template %q has no system prompt", ErrBadVars, t.ID)
	}
	if strings.TrimSpace(t.User) == "" {
		return fmt.Errorf("%w: template %q has no user prompt", ErrBadVars, t.ID)
	}
	if t.TimeoutSeconds <= 0 {
		t.TimeoutSeconds = 30
	}
	if t.Temperature < 0 {
		t.Temperature = 0
	}
	if t.Retry < 0 {
		t.Retry = 0
	}
	if t.Fallback != "" && t.Output == OutputJSON {
		if !jsonLooksValid(strings.TrimSpace(t.Fallback)) {
			return fmt.Errorf("%w: template %q fallback is not valid JSON", ErrBadVars, t.ID)
		}
	}
	return nil
}

// Render materializes a template. Trusted variables are interpolated in the
// body; untrusted ones are sanitized and appended as an UNTRUSTED INPUT
// block. Referencing an untrusted variable in the body fails the render.
func (r *registry) Render(id string, vars map[string]any) (Rendered, error) {
	t, err := r.Get(id)
	if err != nil {
		return Rendered{}, err
	}
	if err := RequireVariables(t, vars); err != nil {
		return Rendered{}, err
	}

	data := map[string]any{}
	for _, v := range t.Variables {
		// Declared trusted variables always exist so optional {{if .x}}
		// guards work; absent ones render as the empty string.
		if !v.Untrusted {
			data[v.Name] = ""
		}
	}
	var untrusted []string
	for _, v := range t.Variables {
		raw, present := vars[v.Name]
		if !present {
			continue
		}
		if v.Untrusted {
			s, _ := raw.(string)
			if s == "" && raw != nil {
				s = fmt.Sprintf("%v", raw)
			}
			s = Sanitize(s, v.MaxRunes)
			if s == "" {
				continue
			}
			label := v.Label
			if label == "" {
				label = v.Name
			}
			untrusted = append(untrusted, label+": "+s)
			continue
		}
		data[v.Name] = raw
	}

	system, err := executeTemplate("system", t.System, data)
	if err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadVars, err)
	}
	user, err := executeTemplate("user", t.User, data)
	if err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadVars, err)
	}
	user = strings.TrimRight(user, "\n")
	user = WrapUntrusted(user, untrusted)

	fallback, err := executeTemplate("fallback", t.Fallback, data)
	if err != nil {
		return Rendered{}, fmt.Errorf("%w: %v", ErrBadVars, err)
	}
	fallback = strings.TrimSpace(fallback)

	return Rendered{
		Task:        t.ID,
		Version:     t.Version,
		Hash:        hashPrompt(system, user),
		System:      system,
		User:        user,
		Temperature: t.Temperature,
		MaxTokens:   t.MaxTokens,
		Timeout:     time.Duration(t.TimeoutSeconds) * time.Second,
		Output:      t.Output,
		Retry:       t.Retry,
		Fallback:    fallback,
	}, nil
}

func executeTemplate(name, text string, data map[string]any) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// jsonLooksValid is a cheap structural check (balanced braces) used to keep
// YAML fallbacks honest without importing encoding/json here.
func jsonLooksValid(s string) bool {
	if len(s) < 2 {
		return false
	}
	var depth int
	inString := false
	escaped := false
	closers := map[rune]bool{'}': true, ']': true}
	openers := map[rune]bool{'{': true, '[': true}
	for _, r := range s {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}
		switch {
		case r == '"':
			inString = true
		case openers[r]:
			depth++
		case closers[r]:
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	if depth != 0 || inString {
		return false
	}
	trimmed := strings.TrimSpace(s)
	return trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}
