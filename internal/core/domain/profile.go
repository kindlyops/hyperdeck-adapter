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

// ControlMode selects how transport commands reach the player: by synthesizing
// keystrokes (the default) or through an out-of-band control API.
type ControlMode string

const (
	ControlKeys ControlMode = "keys" // synthesize keystrokes via the injector
	ControlAPI  ControlMode = "api"  // drive the player through its control API
	ControlUIA  ControlMode = "uia"  // invoke the player's UI Automation controls
)

// APIConfig parameterizes a ControlAPI profile's control channel.
type APIConfig struct {
	Type     string // currently only "vlc_http"
	BaseURL  string // e.g. "http://127.0.0.1:8080"
	Password string // control-API password (VLC HTTP uses Basic auth, empty user)
}

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
	ID            string
	Match         Match
	Control       ControlMode // how transport reaches the player; "" means ControlKeys
	Injection     InjectionMode
	API           APIConfig          // control channel for ControlAPI profiles
	UIA           map[KeyName]string // action -> UI Automation AutomationId, for ControlUIA profiles
	Keymap        Keymap
	PlayToggle    bool // when true, the play key toggles play/pause (e.g. Space in VLC/Example Player): Play suppresses when already playing, and Stop falls back to this key only when no discrete stop key is mapped
	CueOnNavigate bool // when true, next/prev/goto cue the clip paused rather than playing it (e.g. Example Player "pause" playlist mode): navigation leaves the deck stopped so a subsequent Play starts the cued clip
	ClipSource    ClipSourceConfig
	State         StateConfig
	Homing        []Chord
}

// UsesController reports whether the profile is driven by an out-of-band player
// controller (API or UIA) rather than by synthesized keystrokes. The zero value
// ("") and ControlKeys both mean keystroke injection.
func (p Profile) UsesController() bool {
	return p.Control == ControlAPI || p.Control == ControlUIA
}

// HasAction reports whether the profile defines the given transport action. The
// source of truth depends on the control mode: ControlUIA reads the UIA map;
// keystroke and API profiles read the keymap (whose chord values are unused on
// the API path but whose presence still marks which actions exist).
func (p Profile) HasAction(key KeyName) bool {
	if p.Control == ControlUIA {
		_, ok := p.UIA[key]
		return ok
	}
	_, ok := p.Keymap[key]
	return ok
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
