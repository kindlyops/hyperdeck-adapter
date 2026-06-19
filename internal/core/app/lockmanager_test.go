package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

type fakePresenter struct{ last domain.LockState }

func (f *fakePresenter) Present(l domain.LockState) { f.last = l }

type fakeClipSource struct{ clips domain.ClipList }

func (f fakeClipSource) List() (domain.ClipList, error) { return f.clips, nil }

func TestLockManagerLocksOnMatch(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock()}
	csFactory := func(domain.Profile) port.ClipSource { return fakeClipSource{} }
	spFactory := func(domain.Profile) port.StateProbe { return noProbe{} }
	lm := NewLockManager(s, m, profiles, pres, csFactory, spFactory)

	lm.Poll()

	if _, _, ok := s.Active(); !ok {
		t.Fatal("expected lock after matching poll")
	}
	if !pres.last.Locked {
		t.Error("presenter should have been notified of lock")
	}
}

func TestLockManagerUnlocksWhenGone(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	lm := NewLockManager(s, m, []domain.Profile{vlcProfileForLock()}, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })
	lm.Poll()
	m.Windows = nil // player closed
	lm.Poll()
	if _, _, ok := s.Active(); ok {
		t.Error("expected unlock after player disappears")
	}
	if pres.last.Locked {
		t.Error("presenter should reflect unlock")
	}
}

func vlcProfileForLock() domain.Profile {
	return domain.Profile{
		ID:     "vlc",
		Match:  domain.Match{Process: []string{"vlc.exe"}, TitleRegex: "VLC media player"},
		Keymap: domain.Keymap{domain.KeyPlay: {Key: "space"}},
	}
}

type noProbe struct{}

func (noProbe) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StateStopped, false
}

func TestLockManagerPinnedMatchesOnlyActive(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{
		{Process: "vlc.exe", Title: "x - VLC media player"},
		{Process: "Mitti", Title: "Mitti"},
	}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock(), mittiProfileForLock()}
	lm := NewLockManager(s, m, profiles, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })

	lm.SetActive("mitti") // pins mitti even though vlc also matches; triggers Poll

	p, _, ok := s.Active()
	if !ok || p.ID != "mitti" {
		t.Fatalf("expected mitti locked, got ok=%v id=%q", ok, p.ID)
	}
}

func TestLockManagerPinnedNotRunningStaysUnlocked(t *testing.T) {
	m := injector.NewMock()
	m.Windows = []domain.Window{{Process: "vlc.exe", Title: "x - VLC media player"}}
	s := NewSession()
	pres := &fakePresenter{}
	profiles := []domain.Profile{vlcProfileForLock(), mittiProfileForLock()}
	lm := NewLockManager(s, m, profiles, pres,
		func(domain.Profile) port.ClipSource { return fakeClipSource{} },
		func(domain.Profile) port.StateProbe { return noProbe{} })

	lm.SetActive("mitti") // mitti is not in the window list

	if _, _, ok := s.Active(); ok {
		t.Error("expected unlocked when the pinned profile is not running")
	}
}

func mittiProfileForLock() domain.Profile {
	return domain.Profile{
		ID:     "mitti",
		Match:  domain.Match{Process: []string{"Mitti"}},
		Keymap: domain.Keymap{domain.KeyPlay: {Key: "enter"}},
	}
}
