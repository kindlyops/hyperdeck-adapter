//go:build darwin || windows

package tray

import _ "embed"

//go:generate go run gen_icons.go

// Tray icons (PNG-in-ICO, multi-size). Regenerate with the icongen tool. Windows
// shows the icon (it ignores the systray title), so without these the tray entry
// is a blank slot.
//
//go:embed icon_locked.ico
var iconLocked []byte

//go:embed icon_idle.ico
var iconIdle []byte

// lockIcon returns the icon bytes for the current lock state.
func lockIcon(locked bool) []byte {
	if locked {
		return iconLocked
	}
	return iconIdle
}
