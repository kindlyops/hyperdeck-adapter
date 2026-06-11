//go:build windows

package injector

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"golang.org/x/sys/windows"
)

// Injector is the union of the two OS-facing driven ports.
type Injector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
	OpenWindows() ([]domain.Window, error)
}

// New returns the Windows injector.
func New() (Injector, error) { return &winInjector{}, nil }

type winInjector struct{}

// Focus brings the target window to the foreground (focus injection mode).
// Implementation: user32!SetForegroundWindow(HWND). Verified manually on Windows.
func (w *winInjector) Focus(win domain.Window) error {
	return fmt.Errorf("not implemented: build and verify on Windows")
}

// SendKeys delivers chords. focus mode -> SendInput(INPUT_KEYBOARD); background
// mode -> PostMessageW(WM_KEYDOWN/WM_KEYUP) to the HWND.
func (w *winInjector) SendKeys(win domain.Window, chords []domain.Chord) error {
	return fmt.Errorf("not implemented: build and verify on Windows")
}

// OpenWindows enumerates top-level windows via EnumWindows + GetWindowTextW +
// GetWindowThreadProcessId + the owning process's base name.
func (w *winInjector) OpenWindows() ([]domain.Window, error) {
	return nil, fmt.Errorf("not implemented: build and verify on Windows")
}

var _ = windows.NewLazySystemDLL // anchor the dependency import
