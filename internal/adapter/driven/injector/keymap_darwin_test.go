//go:build darwin

package injector

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestKeyCode(t *testing.T) {
	cases := map[string]uint16{
		"space": 49, "n": 45, "p": 35, "s": 1,
		"right": 124, "left": 123, "up": 126, "down": 125,
		"enter": 36, "esc": 53, ".": 47, "period": 47,
	}
	for key, want := range cases {
		got, ok := keyCode(key)
		if !ok || got != want {
			t.Errorf("keyCode(%q) = %d, %v; want %d", key, got, ok, want)
		}
	}
	if _, ok := keyCode("nope"); ok {
		t.Error("unknown key should return ok=false")
	}
}

func TestEventFlags(t *testing.T) {
	if f := eventFlags(nil); f != 0 {
		t.Errorf("no mods = %#x, want 0", f)
	}
	if f := eventFlags([]domain.Modifier{domain.ModCtrl}); f != flagControl {
		t.Errorf("ctrl = %#x, want %#x", f, flagControl)
	}
	if f := eventFlags([]domain.Modifier{domain.ModCmd}); f != flagCommand {
		t.Errorf("cmd = %#x, want %#x", f, flagCommand)
	}
	got := eventFlags([]domain.Modifier{domain.ModCtrl, domain.ModShift})
	if got != flagControl|flagShift {
		t.Errorf("ctrl+shift = %#x, want %#x", got, flagControl|flagShift)
	}
}
