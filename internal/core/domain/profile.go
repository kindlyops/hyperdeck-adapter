package domain

import (
	"regexp"
	"slices"
)

// KeyName is a logical transport action mapped to a chord by a profile.
type KeyName string

const (
	KeyPlay   KeyName = "play"
	KeyStop   KeyName = "stop"
	KeyRecord KeyName = "record"
	KeyNext   KeyName = "next"
	KeyPrev   KeyName = "prev"
)

// InjectionMode selects how keystrokes reach the target window.
type InjectionMode string

const (
	InjectionFocus      InjectionMode = "focus"
	InjectionBackground InjectionMode = "background"
)

// Keymap maps logical actions to concrete chords.
type Keymap map[KeyName]Chord

// Match describes how to recognize the player's window.
type Match struct {
	Process    []string
	TitleRegex string
}

// ClipSourceConfig selects and parameterizes the clip source strategy.
type ClipSourceConfig struct {
	Type  string // "playlist_file" | "positional" | "mitti"
	Path  string
	Count int
}

// StateConfig selects and parameterizes best-effort state detection.
type StateConfig struct {
	Type    string // "title_regex" | "none"
	Playing string // regex applied to the window title when Type == "title_regex"
}

// Profile is one player application's complete mapping definition.
type Profile struct {
	ID         string
	Match      Match
	Injection  InjectionMode
	Keymap     Keymap
	PlayToggle bool // when true, the play key toggles play/pause (e.g. Space in VLC/Example Player): Play suppresses when already playing, and Stop falls back to this key only when no discrete stop key is mapped
	ClipSource ClipSourceConfig
	State      StateConfig
	Homing     []Chord
}

// MatchesWindow reports whether w belongs to this profile.
func (p Profile) MatchesWindow(w Window) bool {
	if !slices.Contains(p.Match.Process, w.Process) {
		return false
	}
	if p.Match.TitleRegex == "" {
		return true
	}
	re, err := regexp.Compile(p.Match.TitleRegex)
	if err != nil {
		return false
	}
	return re.MatchString(w.Title)
}
