package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// defaultProfiles is the seed config written on first run when no profiles file
// exists. It is kept byte-for-byte identical to examples/profiles.yaml (enforced
// by TestDefaultMatchesExample) so the in-repo example and the seeded file agree.
//
//go:embed default_profiles.yaml
var defaultProfiles []byte

// EnsureDefault writes the embedded default profiles file to path when nothing
// exists there yet, creating parent directories as needed. It returns created=true
// only when it wrote the seed file, and is a no-op (created=false, nil) when a file
// already exists at path.
func EnsureDefault(path string) (created bool, err error) {
	switch _, statErr := os.Stat(path); {
	case statErr == nil:
		return false, nil
	case !os.IsNotExist(statErr):
		return false, fmt.Errorf("stat config %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create config dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, defaultProfiles, 0o644); err != nil {
		return false, fmt.Errorf("write default config %q: %w", path, err)
	}
	return true, nil
}
