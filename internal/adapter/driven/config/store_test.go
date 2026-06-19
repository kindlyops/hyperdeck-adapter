package config

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestLoadProfiles(t *testing.T) {
	profiles, err := NewStore("../../../../testdata/profiles.yaml").Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles", len(profiles))
	}
	vlc := profiles[0]
	if vlc.ID != "vlc" || vlc.Injection != domain.InjectionBackground {
		t.Errorf("vlc profile wrong: %+v", vlc)
	}
	if vlc.Keymap[domain.KeyNext].Key != "n" {
		t.Errorf("vlc next key = %+v", vlc.Keymap[domain.KeyNext])
	}
	example := profiles[1]
	if !example.PlayToggle {
		t.Error("example should be play_toggle")
	}
	if example.Keymap[domain.KeyNext].Key != "right" || len(example.Keymap[domain.KeyNext].Mods) != 1 {
		t.Errorf("example next chord = %+v", example.Keymap[domain.KeyNext])
	}
}

func TestLoadAPIProfile(t *testing.T) {
	profiles, err := loadBytes([]byte(`profiles:
  - id: vlc_api
    match: { process: ["vlc.exe"] }
    control: api
    api: { type: vlc_http, base_url: "http://127.0.0.1:8080", password: "pw" }
    keymap: { play: "space", stop: "s", next: "n", prev: "p" }
    play_toggle: true
`))
	if err != nil {
		t.Fatal(err)
	}
	p := profiles[0]
	if p.Control != domain.ControlAPI {
		t.Errorf("control = %q, want api", p.Control)
	}
	if p.API.Type != "vlc_http" || p.API.BaseURL != "http://127.0.0.1:8080" || p.API.Password != "pw" {
		t.Errorf("api config wrong: %+v", p.API)
	}
}

func TestLoadRejectsBadControl(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    control: telepathy
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for bad control mode")
	}
}

func TestLoadRejectsAPIWithoutType(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    control: api
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for api profile missing api.type")
	}
}

func TestLoadRejectsMissingPlayKey(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    injection: focus
    keymap: { next: "n" }
`))
	if err == nil {
		t.Fatal("expected validation error for missing play key")
	}
}

func TestLoadRejectsBadInjection(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"] }
    injection: telepathy
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for bad injection mode")
	}
}

func TestLoadRejectsBadTitleRegex(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: ["x"], title_regex: "[unterminated" }
    injection: focus
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for invalid title_regex")
	}
}

func TestLoadRejectsEmptyProcess(t *testing.T) {
	_, err := loadBytes([]byte(`profiles:
  - id: bad
    match: { process: [] }
    injection: focus
    keymap: { play: "space" }
`))
	if err == nil {
		t.Fatal("expected validation error for empty process list")
	}
}
