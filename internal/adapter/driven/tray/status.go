// Package tray implements port.StatusPresenter and the menu over fyne.io/systray
// on desktop platforms, with a no-op fallback elsewhere (Linux/CI).
package tray

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func statusText(lock domain.LockState) string {
	if !lock.Locked || lock.Profile == nil {
		return "Disconnected — no player"
	}
	return fmt.Sprintf("Locked: %s (%s)", lock.Profile.ID, lock.Window.Title)
}
