//go:build darwin

package injector

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the macOS injector. Requires the Accessibility permission.
func New() (Injector, error) { return &macInjector{}, nil }

type macInjector struct{}

// Focus activates the target application (AX / NSRunningApplication activate).
func (m *macInjector) Focus(win domain.Window) error {
	return fmt.Errorf("not implemented: build and verify on macOS")
}

// SendKeys posts CGEvent keyboard events for each chord.
func (m *macInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	return fmt.Errorf("not implemented: build and verify on macOS")
}

// OpenWindows lists on-screen windows via CGWindowListCopyWindowInfo.
func (m *macInjector) OpenWindows() ([]domain.Window, error) {
	return nil, fmt.Errorf("not implemented: build and verify on macOS")
}
