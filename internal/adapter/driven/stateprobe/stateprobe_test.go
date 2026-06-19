package stateprobe

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestTitleRegexDetectsPlaying(t *testing.T) {
	p := NewTitleRegex(".+ - VLC")
	st, ok := p.Detect(domain.Window{Title: "Movie - VLC"})
	if !ok || st != domain.StatePlaying {
		t.Errorf("got %v %v", st, ok)
	}
	st, ok = p.Detect(domain.Window{Title: "idle"})
	if !ok || st != domain.StateStopped {
		t.Errorf("non-match should report stopped+detected; got %v %v", st, ok)
	}
}

func TestNoneNeverDetects(t *testing.T) {
	_, ok := None{}.Detect(domain.Window{Title: "x"})
	if ok {
		t.Error("none probe must not claim detection")
	}
}

func TestFactory(t *testing.T) {
	none := New(domain.Profile{State: domain.StateConfig{Type: "none"}}, nil)
	if _, ok := none.Detect(domain.Window{}); ok {
		t.Error("factory none should not detect")
	}
}

// fakeNamer returns a fixed element Name for UIA probe tests.
type fakeNamer struct {
	name string
	err  error
}

func (f fakeNamer) Name(uintptr, string) (string, error) { return f.name, f.err }

func TestUIAProbeDetectsFromName(t *testing.T) {
	p := NewUIA(fakeNamer{name: "Pause"}, "TogglePlaybackButton", "Pause")
	if st, ok := p.Detect(domain.Window{}); !ok || st != domain.StatePlaying {
		t.Errorf("Name 'Pause' should be playing+detected; got %v %v", st, ok)
	}
	p = NewUIA(fakeNamer{name: "Play"}, "TogglePlaybackButton", "Pause")
	if st, ok := p.Detect(domain.Window{}); !ok || st != domain.StateStopped {
		t.Errorf("Name 'Play' should be stopped+detected; got %v %v", st, ok)
	}
}

func TestUIAProbeNotDetectedWhenAbsent(t *testing.T) {
	p := NewUIA(fakeNamer{name: ""}, "TogglePlaybackButton", "Pause")
	if _, ok := p.Detect(domain.Window{}); ok {
		t.Error("empty Name (no clip/controls hidden) must report not-detected")
	}
}

func TestFactoryBuildsUIA(t *testing.T) {
	pr := New(domain.Profile{State: domain.StateConfig{Type: "uia", AutomationID: "TogglePlaybackButton", Playing: "Pause"}}, fakeNamer{name: "Pause"})
	if st, ok := pr.Detect(domain.Window{}); !ok || st != domain.StatePlaying {
		t.Errorf("factory uia probe should detect playing; got %v %v", st, ok)
	}
}
