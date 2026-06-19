package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func lockedSession(p domain.Profile, clips domain.ClipList) *Session {
	s := NewSession()
	s.Lock(p, domain.Window{Process: "x"}, nil, nil)
	s.SetClips(clips)
	return s
}

func sentKeys(m *injector.Mock) []string {
	var out []string
	for _, s := range m.Sent {
		for _, c := range s.Chords {
			out = append(out, c.Key)
		}
	}
	return out
}

func discreteProfile() domain.Profile {
	return domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap: domain.Keymap{
			domain.KeyPlay: {Key: "space"},
			domain.KeyStop: {Key: "s"},
			domain.KeyNext: {Key: "n"},
			domain.KeyPrev: {Key: "p"},
		},
	}
}

func toggleProfile() domain.Profile {
	return domain.Profile{
		ID:         "example",
		Injection:  domain.InjectionFocus,
		PlayToggle: true,
		Keymap: domain.Keymap{
			domain.KeyPlay: {Key: "space"},
			domain.KeyNext: {Key: "right", Mods: []domain.Modifier{domain.ModCtrl}},
			domain.KeyPrev: {Key: "left", Mods: []domain.Modifier{domain.ModCtrl}},
		},
	}
}

func TestPlayStopDiscrete(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(discreteProfile(), nil), m)
	if err := d.Play(); err != nil {
		t.Fatal(err)
	}
	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}
	got := sentKeys(m)
	want := []string{"space", "s"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestToggleSuppressesRedundant(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(toggleProfile(), nil)
	d := NewVirtualDeck(s, m)
	_ = d.Play()
	_ = d.Play()
	_ = d.Stop()
	_ = d.Stop()
	got := sentKeys(m)
	if len(got) != 2 {
		t.Fatalf("expected 2 keypresses, got %v", got)
	}
}

func TestToggleFocusesFirst(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(toggleProfile(), nil), m)
	_ = d.Play()
	if len(m.Focused) != 1 {
		t.Errorf("focus mode should focus before sending; Focused=%v", m.Focused)
	}
}

func TestGotoComputesDelta(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}, {ID: 5}})
	d := NewVirtualDeck(s, m)
	if err := d.Goto(4); err != nil {
		t.Fatal(err)
	}
	if got := sentKeys(m); len(got) != 3 || got[0] != "n" {
		t.Errorf("goto 4 keys = %v, want 3x n", got)
	}
	if s.CurrentClip() != 4 {
		t.Errorf("current clip = %d, want 4", s.CurrentClip())
	}
	m.Sent = nil
	if err := d.Goto(2); err != nil {
		t.Fatal(err)
	}
	if got := sentKeys(m); len(got) != 2 || got[0] != "p" {
		t.Errorf("goto 2 keys = %v, want 2x p", got)
	}
}

func TestGotoClampsToRange(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(discreteProfile(), domain.ClipList{{ID: 1}, {ID: 2}})
	d := NewVirtualDeck(s, m)
	if err := d.Goto(99); err != nil {
		t.Fatal(err)
	}
	if s.CurrentClip() != 2 {
		t.Errorf("clamped current = %d, want 2", s.CurrentClip())
	}
}

func TestRecordUndefinedIsNoop(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(lockedSession(discreteProfile(), nil), m)
	if err := d.Record(); err != nil {
		t.Fatalf("record with no mapping should be a silent no-op, got %v", err)
	}
	if len(m.Sent) != 0 {
		t.Errorf("record should send nothing, sent %v", m.Sent)
	}
}

func TestCommandsWithoutLockError(t *testing.T) {
	m := injector.NewMock()
	d := NewVirtualDeck(NewSession(), m)
	if err := d.Play(); err != ErrNotLocked {
		t.Errorf("expected ErrNotLocked, got %v", err)
	}
}

// cueModeProfile models Example Player in playlist "pause" (cue) mode: a toggle play
// key plus cue_on_navigate, so navigating cues the clip paused.
func cueModeProfile() domain.Profile {
	p := toggleProfile()
	p.ID = "example"
	p.CueOnNavigate = true
	return p
}

func TestCueOnNavigateLeavesDeckStoppedSoPlayStarts(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(cueModeProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}})
	d := NewVirtualDeck(s, m)
	_ = d.Play() // stopped -> playing: space
	_ = d.Next() // navigates (ctrl+right) and cues paused -> state goes stopped
	if s.State() != domain.StateStopped {
		t.Fatalf("cue_on_navigate: Next should leave the deck cued/stopped, got %v", s.State())
	}
	_ = d.Play() // stopped -> playing again: must re-emit space to start the cued clip
	got := sentKeys(m)
	want := []string{"space", "right", "space"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestWithoutCueOnNavigatePlayStaysSuppressed(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(toggleProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}})
	d := NewVirtualDeck(s, m)
	_ = d.Play() // space
	_ = d.Next() // right; no cue, state stays playing
	_ = d.Play() // suppressed: already modeled as playing
	got := sentKeys(m)
	want := []string{"space", "right"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("without cue_on_navigate, second Play should be suppressed; keys = %v, want %v", got, want)
	}
}

func TestCueOnNavigateGotoAlsoCues(t *testing.T) {
	m := injector.NewMock()
	s := lockedSession(cueModeProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
	d := NewVirtualDeck(s, m)
	s.SetState(domain.StatePlaying)
	if err := d.Goto(3); err != nil {
		t.Fatal(err)
	}
	if s.State() != domain.StateStopped {
		t.Errorf("cue_on_navigate: Goto should leave the deck cued/stopped, got %v", s.State())
	}
}
