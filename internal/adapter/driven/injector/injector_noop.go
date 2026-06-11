//go:build !windows && !darwin

package injector

import (
	"log/slog"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// New returns the platform injector. On unsupported platforms it logs and no-ops,
// so the protocol server still runs for development against a controller.
func New() (Injector, error) {
	slog.Warn("key injection is not supported on this platform; running in no-op mode")
	return noop{}, nil
}

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

type noop struct{}

func (noop) Focus(domain.Window) error                    { return nil }
func (noop) SendKeys(domain.Window, []domain.Chord) error { return nil }
func (noop) OpenWindows() ([]domain.Window, error)        { return nil, nil }
