package domain

import (
	"fmt"
	"strings"
)

// Modifier is a keyboard modifier key.
type Modifier string

const (
	ModCtrl  Modifier = "ctrl"
	ModAlt   Modifier = "alt"
	ModShift Modifier = "shift"
	ModCmd   Modifier = "cmd"
)

var knownModifiers = map[string]Modifier{
	"ctrl": ModCtrl, "alt": ModAlt, "shift": ModShift, "cmd": ModCmd,
}

// Chord is a single keystroke: zero or more modifiers plus one base key.
type Chord struct {
	Mods []Modifier
	Key  string
}

// ParseChord parses strings like "ctrl+right" or "space" into a Chord.
// Matching is case-insensitive. The base key must be the last token.
func ParseChord(s string) (Chord, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	if len(parts) == 0 || parts[len(parts)-1] == "" {
		return Chord{}, fmt.Errorf("parse chord %q: empty key", s)
	}
	var c Chord
	for i, p := range parts {
		if p == "" {
			return Chord{}, fmt.Errorf("parse chord %q: empty token", s)
		}
		if i == len(parts)-1 {
			c.Key = p
			continue
		}
		mod, ok := knownModifiers[p]
		if !ok {
			return Chord{}, fmt.Errorf("parse chord %q: unknown modifier %q", s, p)
		}
		c.Mods = append(c.Mods, mod)
	}
	return c, nil
}
