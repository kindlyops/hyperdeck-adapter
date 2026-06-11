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
	none := New(domain.Profile{State: domain.StateConfig{Type: "none"}})
	if _, ok := none.Detect(domain.Window{}); ok {
		t.Error("factory none should not detect")
	}
}
