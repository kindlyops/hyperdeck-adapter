package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultMatchesExample guards against the embedded seed config drifting from
// the human-facing examples/profiles.yaml: both must stay byte-for-byte identical.
func TestDefaultMatchesExample(t *testing.T) {
	example, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "examples", "profiles.yaml"))
	if err != nil {
		t.Fatalf("read examples/profiles.yaml: %v", err)
	}
	if !bytes.Equal(example, defaultProfiles) {
		t.Error("embedded default_profiles.yaml differs from examples/profiles.yaml; keep them in sync")
	}
}

// TestEnsureDefaultSeedsAndIsIdempotent verifies first-run seeding creates the
// file (and parent dirs) and that a second call leaves an existing file alone.
func TestEnsureDefaultSeedsAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "profiles.yaml")

	created, err := EnsureDefault(path)
	if err != nil {
		t.Fatalf("EnsureDefault first call: %v", err)
	}
	if !created {
		t.Fatal("expected created=true when file is absent")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seeded file: %v", err)
	}
	if !bytes.Equal(got, defaultProfiles) {
		t.Error("seeded file does not match embedded default")
	}

	// Mutate the file; a second call must not overwrite it.
	if err := os.WriteFile(path, []byte("profiles: []\n"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	created, err = EnsureDefault(path)
	if err != nil {
		t.Fatalf("EnsureDefault second call: %v", err)
	}
	if created {
		t.Error("expected created=false when file already exists")
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read file: %v", err)
	}
	if string(got) != "profiles: []\n" {
		t.Error("EnsureDefault overwrote an existing config")
	}
}
