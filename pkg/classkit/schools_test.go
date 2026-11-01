package classkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchoolRegistry(t *testing.T) {
	want := map[string]string{
		"martial": "Martial",
		"fire":    "Fire",
		"ice":     "Ice",
		"arcane":  "Arcane",
		"divine":  "Divine",
		"nature":  "Nature",
		"shadow":  "Shadow",
	}
	for id, name := range want {
		if got := SchoolName(id); got != name {
			t.Errorf("SchoolName(%q) = %q, want %q", id, got, name)
		}
		if !KnownSchool(id) {
			t.Errorf("KnownSchool(%q) = false", id)
		}
	}
	if KnownSchool("") || SchoolName("") != "" {
		t.Fatal("empty school must be unknown and nameless")
	}
	if KnownSchool("necromancy") || SchoolName("necromancy") != "" {
		t.Fatal("unregistered school must be unknown and nameless")
	}
}

func TestLoadDirSchoolValid(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: ember
name: Ember
school: fire
hp_multiplier: 0.80
skills:
  - id: ember_sear
    name: Sear
    effect: sear
`)
	n, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if n != 1 {
		t.Fatalf("loaded %d, want 1", n)
	}
	d := Lookup("ember")
	if d == nil || d.School != "fire" {
		t.Fatalf("class = %+v", d)
	}
	if !FromPack() {
		t.Fatal("valid directory should install as a pack")
	}
}

func TestLoadDirSchoolUnknown(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: hex
name: Hex
school: necromancy
skills:
  - id: hex_slam
    name: Slam
    effect: slam
`)
	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("unknown school must fail the load")
	}
	if !strings.Contains(err.Error(), "unknown school") || !strings.Contains(err.Error(), "necromancy") {
		t.Fatalf("err = %v", err)
	}
	if Lookup("hex") != nil {
		t.Fatal("a failed load must not install any class")
	}
}

func TestLoadDirSchoolOmitted(t *testing.T) {
	t.Cleanup(func() { restoreTestPack(t) })
	dir := writeClassYAML(t, `
id: plain
name: Plain
skills:
  - id: plain_slam
    name: Slam
    effect: slam
`)
	n, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if n != 1 {
		t.Fatalf("loaded %d, want 1", n)
	}
	d := Lookup("plain")
	if d == nil || d.School != "" {
		t.Fatalf("class = %+v", d)
	}
}

func writeClassYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "class.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write class yaml: %v", err)
	}
	return dir
}

// restoreTestPack puts the package testdata catalog back after a test that
// replaced it. Tests in this package run against the testdata pack (load.go
// loads it in init) unless they deliberately swap it out.
func restoreTestPack(t *testing.T) {
	t.Helper()
	dir := findUp("pkg/classkit/testdata/classes")
	if dir == "" {
		t.Fatal("testdata class directory not found")
	}
	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("restore testdata pack: %v", err)
	}
}
