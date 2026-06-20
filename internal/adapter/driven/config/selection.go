package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SelectionStore persists the pinned profile id (empty = Auto / match-any) to a
// JSON file kept separate from profiles.yaml, so the user's commented config is
// never rewritten.
type SelectionStore struct{ path string }

// NewSelectionStore returns a SelectionStore backed by path.
func NewSelectionStore(path string) *SelectionStore { return &SelectionStore{path: path} }

type selectionSchema struct {
	ActiveProfile string `json:"active_profile"`
}

// Load returns the pinned profile id. A missing file is not an error and yields
// "" (Auto); a malformed file returns an error.
func (s *SelectionStore) Load() (string, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read selection %q: %w", s.path, err)
	}
	var sel selectionSchema
	if err := json.Unmarshal(data, &sel); err != nil {
		return "", fmt.Errorf("parse selection %q: %w", s.path, err)
	}
	return sel.ActiveProfile, nil
}

// Save writes the pinned profile id (empty clears to Auto), creating parent
// directories as needed.
func (s *SelectionStore) Save(id string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create selection dir for %q: %w", s.path, err)
	}
	data, err := json.Marshal(selectionSchema{ActiveProfile: id})
	if err != nil {
		return fmt.Errorf("encode selection: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write selection %q: %w", s.path, err)
	}
	return nil
}
