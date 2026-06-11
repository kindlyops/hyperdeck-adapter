package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestSessionLockUnlock(t *testing.T) {
	s := NewSession()
	if _, _, ok := s.Active(); ok {
		t.Fatal("new session should be unlocked")
	}
	p := domain.Profile{ID: "vlc"}
	w := domain.Window{Process: "vlc.exe"}
	s.Lock(p, w, nil, nil)
	gotP, gotW, ok := s.Active()
	if !ok || gotP.ID != "vlc" || gotW.Process != "vlc.exe" {
		t.Fatalf("Active = %+v %+v %v", gotP, gotW, ok)
	}
	s.Unlock()
	if _, _, ok := s.Active(); ok {
		t.Fatal("session should be unlocked after Unlock")
	}
}

func TestSessionStateAndClip(t *testing.T) {
	s := NewSession()
	s.SetState(domain.StatePlaying)
	if s.State() != domain.StatePlaying {
		t.Error("state not set")
	}
	s.SetClips(domain.ClipList{{ID: 1}, {ID: 2}})
	if s.Clips().Len() != 2 {
		t.Error("clips not set")
	}
	s.SetCurrentClip(2)
	if s.CurrentClip() != 2 {
		t.Error("current clip not set")
	}
}
