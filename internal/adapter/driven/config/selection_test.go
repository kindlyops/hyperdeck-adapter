package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selection.json")
	s := NewSelectionStore(path)
	if err := s.Save("vlc"); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "vlc" {
		t.Errorf("got %q want %q", got, "vlc")
	}
}

func TestSelectionStoreMissingFileIsAuto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := NewSelectionStore(path).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != "" {
		t.Errorf("got %q want empty (Auto)", got)
	}
}

func TestSelectionStoreMalformedFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSelectionStore(path).Load(); err == nil {
		t.Error("expected error for malformed selection file")
	}
}
