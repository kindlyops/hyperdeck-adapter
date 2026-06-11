//go:build !darwin && !windows

package tray

import (
	"log/slog"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Tray is a headless no-op presenter for platforms without a system tray
// (Linux/CI). It logs status changes and blocks in Run so the process stays up.
type Tray struct {
	onRehome func()
	onQuit   func()
	last     domain.LockState
}

// New returns a no-op Tray with the same API as the desktop implementation.
func New(onRehome, onQuit func()) *Tray {
	return &Tray{onRehome: onRehome, onQuit: onQuit}
}

// Present logs the current lock status.
func (t *Tray) Present(lock domain.LockState) {
	t.last = lock
	slog.Info("status", "text", statusText(lock))
}

// Run blocks forever; there is no tray UI on this platform.
func (t *Tray) Run() { select {} }

var _ port.StatusPresenter = (*Tray)(nil)
