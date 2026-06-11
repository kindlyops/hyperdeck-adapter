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
	if !example.PlayStopToggle {
		t.Error("example should be play_stop_toggle")
	}
	if example.Keymap[domain.KeyNext].Key != "right" || len(example.Keymap[domain.KeyNext].Mods) != 1 {
		t.Errorf("example next chord = %+v", example.Keymap[domain.KeyNext])
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
