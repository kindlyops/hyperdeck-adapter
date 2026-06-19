package app

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// mockController records Control calls for ControlAPI dispatch tests.
type mockController struct {
	keys []domain.KeyName
	err  error
}

func (m *mockController) Control(_ domain.Profile, _ domain.Window, key domain.KeyName) error {
	if m.err != nil {
		return m.err
	}
	m.keys = append(m.keys, key)
	return nil
}

func apiProfile() domain.Profile {
	p := discreteProfile()
	p.Control = domain.ControlAPI
	p.API = domain.APIConfig{Type: "vlc_http"}
	return p
}

func TestAPIControlRoutesToController(t *testing.T) {
	m := injector.NewMock()
	ctrl := &mockController{}
	d := NewVirtualDeck(lockedSession(apiProfile(), nil), m, WithController(ctrl))

	if err := d.Play(); err != nil {
		t.Fatal(err)
	}
	if err := d.Stop(); err != nil {
		t.Fatal(err)
	}

	want := []domain.KeyName{domain.KeyPlay, domain.KeyStop}
	if len(ctrl.keys) != 2 || ctrl.keys[0] != want[0] || ctrl.keys[1] != want[1] {
		t.Errorf("controller keys = %v, want %v", ctrl.keys, want)
	}
	if len(m.Sent) != 0 || len(m.Focused) != 0 {
		t.Errorf("api control must not touch the injector; sent=%v focused=%v", m.Sent, m.Focused)
	}
}

func TestAPIControlGotoIssuesPerStep(t *testing.T) {
	ctrl := &mockController{}
	s := lockedSession(apiProfile(), domain.ClipList{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4}})
	d := NewVirtualDeck(s, injector.NewMock(), WithController(ctrl))

	if err := d.Goto(4); err != nil {
		t.Fatal(err)
	}
	if len(ctrl.keys) != 3 {
		t.Fatalf("goto 4 from 1 should issue 3 commands, got %v", ctrl.keys)
	}
	for i, k := range ctrl.keys {
		if k != domain.KeyNext {
			t.Errorf("command %d = %q, want next", i, k)
		}
	}
}

func TestAPIControlRehomeStops(t *testing.T) {
	ctrl := &mockController{}
	s := lockedSession(apiProfile(), nil)
	d := NewVirtualDeck(s, injector.NewMock(), WithController(ctrl))

	if err := d.Rehome(); err != nil {
		t.Fatal(err)
	}
	if len(ctrl.keys) != 1 || ctrl.keys[0] != domain.KeyStop {
		t.Errorf("rehome should issue stop, got %v", ctrl.keys)
	}
	if s.State() != domain.StateStopped || s.CurrentClip() != 1 {
		t.Errorf("rehome should reset state/clip; state=%v clip=%d", s.State(), s.CurrentClip())
	}
}

func TestAPIControlWithoutControllerErrors(t *testing.T) {
	d := NewVirtualDeck(lockedSession(apiProfile(), nil), injector.NewMock())
	if err := d.Play(); err == nil {
		t.Error("api profile without a configured controller should error")
	}
}
