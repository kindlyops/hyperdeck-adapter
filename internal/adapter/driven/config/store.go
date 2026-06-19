// Package config implements port.ProfileStore over a YAML file.
package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Store loads profiles from a YAML file path.
type Store struct{ path string }

// NewStore returns a ProfileStore reading from path.
func NewStore(path string) *Store { return &Store{path: path} }

// Load reads and validates the profile file.
func (s *Store) Load() ([]domain.Profile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", s.path, err)
	}
	return loadBytes(data)
}

type fileSchema struct {
	Profiles []profileSchema `yaml:"profiles"`
}

type profileSchema struct {
	ID        string            `yaml:"id"`
	Match     matchSchema       `yaml:"match"`
	Control   string            `yaml:"control"`
	Injection string            `yaml:"injection"`
	API       apiSchema         `yaml:"api"`
	UIA       map[string]string `yaml:"uia"`
	Keymap    map[string]string `yaml:"keymap"`
	Toggle    bool              `yaml:"play_toggle"`
	CueNav    bool              `yaml:"cue_on_navigate"`
	Clip      clipSchema        `yaml:"clip_source"`
	State     stateSchema       `yaml:"state"`
	Homing    []string          `yaml:"homing"`
}

type apiSchema struct {
	Type     string `yaml:"type"`
	BaseURL  string `yaml:"base_url"`
	Password string `yaml:"password"`
}

type matchSchema struct {
	Process    []string `yaml:"process"`
	TitleRegex string   `yaml:"title_regex"`
}

type clipSchema struct {
	Type  string `yaml:"type"`
	Path  string `yaml:"path"`
	Count int    `yaml:"count"`
}

type stateSchema struct {
	Type    string `yaml:"type"`
	Playing string `yaml:"playing"`
}

func loadBytes(data []byte) ([]domain.Profile, error) {
	var f fileSchema
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	profiles := make([]domain.Profile, 0, len(f.Profiles))
	for _, ps := range f.Profiles {
		p, err := convert(ps)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

func convert(ps profileSchema) (domain.Profile, error) {
	if ps.ID == "" {
		return domain.Profile{}, fmt.Errorf("profile missing id")
	}
	control := domain.ControlMode(ps.Control)
	if control == "" {
		control = domain.ControlKeys
	}
	if control != domain.ControlKeys && control != domain.ControlAPI && control != domain.ControlUIA {
		return domain.Profile{}, fmt.Errorf("profile %q: invalid control %q (want keys|api|uia)", ps.ID, ps.Control)
	}
	// Injection only governs keystroke control; api profiles may omit it.
	mode := domain.InjectionMode(ps.Injection)
	if control == domain.ControlKeys && mode != domain.InjectionFocus && mode != domain.InjectionBackground {
		return domain.Profile{}, fmt.Errorf("profile %q: invalid injection %q (want focus|background)", ps.ID, ps.Injection)
	}
	var api domain.APIConfig
	if control == domain.ControlAPI {
		if ps.API.Type != "vlc_http" {
			return domain.Profile{}, fmt.Errorf("profile %q: invalid api.type %q (want vlc_http)", ps.ID, ps.API.Type)
		}
		api = domain.APIConfig{Type: ps.API.Type, BaseURL: ps.API.BaseURL, Password: ps.API.Password}
	}
	var uia map[domain.KeyName]string
	if control == domain.ControlUIA {
		if len(ps.UIA) == 0 {
			return domain.Profile{}, fmt.Errorf("profile %q: uia control requires a uia: map of action -> AutomationId", ps.ID)
		}
		uia = make(map[domain.KeyName]string, len(ps.UIA))
		for name, aid := range ps.UIA {
			if aid == "" {
				return domain.Profile{}, fmt.Errorf("profile %q: uia.%s has an empty AutomationId", ps.ID, name)
			}
			uia[domain.KeyName(name)] = aid
		}
	}
	if len(ps.Match.Process) == 0 {
		return domain.Profile{}, fmt.Errorf("profile %q: match.process must list at least one process name", ps.ID)
	}
	for _, name := range ps.Match.Process {
		if name == "" {
			return domain.Profile{}, fmt.Errorf("profile %q: match.process contains an empty entry", ps.ID)
		}
	}
	if ps.Match.TitleRegex != "" {
		if _, err := regexp.Compile(ps.Match.TitleRegex); err != nil {
			return domain.Profile{}, fmt.Errorf("profile %q: invalid title_regex: %w", ps.ID, err)
		}
	}
	keymap := domain.Keymap{}
	for name, spec := range ps.Keymap {
		chord, err := domain.ParseChord(spec)
		if err != nil {
			return domain.Profile{}, fmt.Errorf("profile %q key %q: %w", ps.ID, name, err)
		}
		keymap[domain.KeyName(name)] = chord
	}
	if control == domain.ControlUIA {
		if _, ok := uia[domain.KeyPlay]; !ok {
			return domain.Profile{}, fmt.Errorf("profile %q: missing required uia.play AutomationId", ps.ID)
		}
	} else if _, ok := keymap[domain.KeyPlay]; !ok {
		return domain.Profile{}, fmt.Errorf("profile %q: missing required 'play' key", ps.ID)
	}
	var homing []domain.Chord
	for _, spec := range ps.Homing {
		chord, err := domain.ParseChord(spec)
		if err != nil {
			return domain.Profile{}, fmt.Errorf("profile %q homing %q: %w", ps.ID, spec, err)
		}
		homing = append(homing, chord)
	}
	return domain.Profile{
		ID:            ps.ID,
		Match:         domain.Match{Process: ps.Match.Process, TitleRegex: ps.Match.TitleRegex},
		Control:       control,
		Injection:     mode,
		API:           api,
		UIA:           uia,
		Keymap:        keymap,
		PlayToggle:    ps.Toggle,
		CueOnNavigate: ps.CueNav,
		ClipSource:    domain.ClipSourceConfig{Type: ps.Clip.Type, Path: ps.Clip.Path, Count: ps.Clip.Count},
		State:         domain.StateConfig{Type: ps.State.Type, Playing: ps.State.Playing},
		Homing:        homing,
	}, nil
}

var _ port.ProfileStore = (*Store)(nil)
