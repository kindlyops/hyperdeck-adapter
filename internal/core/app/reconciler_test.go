package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

type playingProbe struct{}

func (playingProbe) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StatePlaying, true
}

func TestReconcilerRefreshesClipsAndState(t *testing.T) {
	s := NewSession()
	s.Lock(discreteProfile(), domain.Window{}, fakeClipSource{clips: domain.ClipList{{ID: 1, Name: "a"}}}, playingProbe{})
	r := NewReconciler(s)
	r.Tick()
	if s.Clips().Len() != 1 {
		t.Errorf("clips not refreshed: %v", s.Clips())
	}
	if s.State() != domain.StatePlaying {
		t.Errorf("state not corrected to playing")
	}
}

func TestReconcilerNoopWhenUnlocked(t *testing.T) {
	s := NewSession()
	r := NewReconciler(s)
	r.Tick() // must not panic with nil clip source / probe
}
