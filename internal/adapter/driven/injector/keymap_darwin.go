//go:build darwin

package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// CGEventFlags modifier masks (stable values from CGEventTypes.h). Kept in pure
// Go so the chord→keycode/flags mapping is unit-testable without invoking cgo.
const (
	flagShift   uint64 = 1 << 17
	flagControl uint64 = 1 << 18
	flagAlt     uint64 = 1 << 19
	flagCommand uint64 = 1 << 20
)

// eventFlags converts chord modifiers into a CGEventFlags bitmask.
func eventFlags(mods []domain.Modifier) uint64 {
	var f uint64
	for _, m := range mods {
		switch m {
		case domain.ModShift:
			f |= flagShift
		case domain.ModCtrl:
			f |= flagControl
		case domain.ModAlt:
			f |= flagAlt
		case domain.ModCmd:
			f |= flagCommand
		}
	}
	return f
}

// darwinKeyCodes maps base key names to ANSI virtual key codes (kVK_* from
// Carbon HIToolbox Events.h). Codes are physical positions on a US ANSI layout.
var darwinKeyCodes = map[string]uint16{
	"space": 49, "return": 36, "enter": 36, "tab": 48, "esc": 53, "escape": 53,
	"delete": 51, "backspace": 51,
	"left": 123, "right": 124, "down": 125, "up": 126,
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
	"b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17,
	"o": 31, "u": 32, "i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
	"1": 18, "2": 19, "3": 20, "4": 21, "5": 23, "6": 22, "7": 26, "8": 28, "9": 25, "0": 29,
}

// keyCode returns the macOS virtual key code for a base key name.
func keyCode(key string) (uint16, bool) {
	c, ok := darwinKeyCodes[key]
	return c, ok
}
