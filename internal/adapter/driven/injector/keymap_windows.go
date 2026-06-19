//go:build windows

package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// Windows Virtual-Key codes (winuser.h). Kept in pure Go — no Win32 calls — so the
// chord→VK mapping is unit-testable without invoking any syscall, mirroring the
// keyCode/eventFlags split in keymap_darwin.go.
//
// Letters VK_A..VK_Z are 0x41..0x5A (ASCII uppercase) and digits VK_0..VK_9 are
// 0x30..0x39, so those are computed in keyCode rather than tabulated here.
var windowsKeyCodes = map[string]uint16{
	"space": 0x20, "enter": 0x0D, "return": 0x0D, "tab": 0x09, "esc": 0x1B, "escape": 0x1B,
	"backspace": 0x08, "delete": 0x2E,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
	"period": 0xBE, ".": 0xBE, "comma": 0xBC, ",": 0xBC,
}

// keyCode returns the Windows virtual-key code for a base key name.
func keyCode(key string) (uint16, bool) {
	if c, ok := windowsKeyCodes[key]; ok {
		return c, true
	}
	if len(key) == 1 {
		ch := key[0]
		if ch >= 'a' && ch <= 'z' {
			return uint16(ch-'a') + 0x41, true
		}
		if ch >= '0' && ch <= '9' {
			return uint16(ch-'0') + 0x30, true
		}
	}
	return 0, false
}

// modifierVK maps a chord modifier to its Windows virtual-key code.
func modifierVK(m domain.Modifier) (uint16, bool) {
	switch m {
	case domain.ModCtrl:
		return 0x11, true // VK_CONTROL
	case domain.ModShift:
		return 0x10, true // VK_SHIFT
	case domain.ModAlt:
		return 0x12, true // VK_MENU
	case domain.ModCmd:
		return 0x5B, true // VK_LWIN (rare in app shortcuts; included for parity)
	}
	return 0, false
}
