package domain

import (
	"reflect"
	"testing"
)

func TestParseChord(t *testing.T) {
	cases := []struct {
		in   string
		want Chord
	}{
		{"space", Chord{Key: "space"}},
		{"s", Chord{Key: "s"}},
		{"ctrl+right", Chord{Mods: []Modifier{ModCtrl}, Key: "right"}},
		{"cmd+esc", Chord{Mods: []Modifier{ModCmd}, Key: "esc"}},
		{"CTRL+Shift+Up", Chord{Mods: []Modifier{ModCtrl, ModShift}, Key: "up"}},
	}
	for _, c := range cases {
		got, err := ParseChord(c.in)
		if err != nil {
			t.Fatalf("ParseChord(%q) error: %v", c.in, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseChord(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseChordErrors(t *testing.T) {
	for _, in := range []string{"", "ctrl+", "+s", "bogusmod+x"} {
		if _, err := ParseChord(in); err == nil {
			t.Errorf("ParseChord(%q) expected error, got nil", in)
		}
	}
}
