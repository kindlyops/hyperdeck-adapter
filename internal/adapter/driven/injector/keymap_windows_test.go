//go:build windows

package injector

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestKeyCode(t *testing.T) {
	cases := map[string]uint16{
		"space": 0x20, "n": 0x4E, "p": 0x50, "s": 0x53,
		"right": 0x27, "left": 0x25, "up": 0x26, "down": 0x28,
		"enter": 0x0D, "esc": 0x1B, ".": 0xBE, "period": 0xBE,
		"a": 0x41, "z": 0x5A, "0": 0x30, "9": 0x39,
	}
	for key, want := range cases {
		got, ok := keyCode(key)
		if !ok || got != want {
			t.Errorf("keyCode(%q) = %#x, %v; want %#x", key, got, ok, want)
		}
	}
	if _, ok := keyCode("nope"); ok {
		t.Error("unknown key should return ok=false")
	}
}

func TestModifierVK(t *testing.T) {
	cases := map[domain.Modifier]uint16{
		domain.ModCtrl:  0x11,
		domain.ModShift: 0x10,
		domain.ModAlt:   0x12,
		domain.ModCmd:   0x5B,
	}
	for mod, want := range cases {
		got, ok := modifierVK(mod)
		if !ok || got != want {
			t.Errorf("modifierVK(%q) = %#x, %v; want %#x", mod, got, ok, want)
		}
	}
	if _, ok := modifierVK(domain.Modifier("bogus")); ok {
		t.Error("unknown modifier should return ok=false")
	}
}
