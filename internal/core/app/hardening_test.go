package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

func TestToggleStopEmitsPlayKey(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(toggleProfile(), nil), m)
	_ = d.Play() // -> space
	_ = d.Stop() // toggle stop must re-emit the PLAY key (space), not a stop key
	got := sentKeys(m)
	if len(got) != 2 || got[1] != "space" {
		t.Errorf("toggle stop should emit play key 'space'; got %v", got)
	}
}

func TestNextPrevClampAtBoundaries(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}})
	d := NewVirtualDeck(s, m)
	// current starts at 1; Prev should clamp at 1.
	if err := d.Prev(); err != nil {
		t.Fatal(err)
	}
	if s.CurrentClip() != 1 {
		t.Errorf("Prev at first clip should stay 1, got %d", s.CurrentClip())
	}
	s.SetCurrentClip(2)
	if err := d.Next(); err != nil {
		t.Fatal(err)
	}
	if s.CurrentClip() != 2 {
		t.Errorf("Next at last clip should stay 2, got %d", s.CurrentClip())
	}
}

func TestNavigationEmptyClipsNoop(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(discreteProfile(), nil), m)
	if err := d.Goto(3); err != nil {
		t.Fatal(err)
	}
	if err := d.Next(); err != nil {
		t.Fatal(err)
	}
	if len(m.Sent) != 0 {
		t.Errorf("navigation with no clips should emit nothing, got %v", m.Sent)
	}
}

func TestRehomeRunsHomingAndResets(t *testing.T) {
	m := injector.NewMock()
	p := domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionFocus,
		Keymap:    domain.Keymap{domain.KeyPlay: {Key: "space"}},
		Homing:    []domain.Chord{{Key: "s"}},
	}
	s := lockedSession(p, domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}})
	s.SetState(domain.StatePlaying)
	s.SetCurrentClip(3)
	d := NewVirtualDeck(s, m)
	if err := d.Rehome(); err != nil {
		t.Fatal(err)
	}
	if len(m.Focused) != 1 {
		t.Errorf("focus-mode rehome should focus, got %v", m.Focused)
	}
	if got := sentKeys(m); len(got) != 1 || got[0] != "s" {
		t.Errorf("rehome should send homing sequence [s], got %v", got)
	}
	if s.State() != domain.StateStopped {
		t.Error("rehome should reset state to stopped")
	}
	if s.CurrentClip() != 1 {
		t.Errorf("rehome should reset current clip to 1, got %d", s.CurrentClip())
	}
}

func TestLockManagerRelocksOnProfileChange(t *testing.T) {
	m := injector.NewMock()
	vlc := vlcProfileForLock()
	example := domain.Profile{
		ID:     "example",
		Match:  domain.Match{Process: []string{"Example Player"}},
		Keymap: domain.Keymap{domain.KeyPlay: {Key: "space"}},
	}
	s := NewSession()
	pres := &fakePresenter{}
	lm := NewLockManager(s, m, []domain.Profile{vlc, example}, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })

	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	lm.Poll()
	if p, _, ok := s.Active(); !ok || p.ID != "vlc" {
		t.Fatalf("expected vlc lock, got %+v ok=%v", p, ok)
	}

	// vlc closes, Example Player appears -> should re-lock to example.
	m.Windows = []domain.Window{{Process: "Example Player", Title: "Example Player"}}
	lm.Poll()
	if p, _, ok := s.Active(); !ok || p.ID != "example" {
		t.Fatalf("expected re-lock to example, got %+v ok=%v", p, ok)
	}
	if !pres.last.Locked || pres.last.Profile == nil || pres.last.Profile.ID != "example" {
		t.Errorf("presenter should reflect example lock, got %+v", pres.last)
	}
}
