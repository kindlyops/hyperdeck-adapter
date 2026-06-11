package hyperdeck

import (
	"strings"
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func newTestDeck() *app.VirtualDeck {
	s := app.NewSession()
	s.Lock(domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap:    domain.Keymap{domain.KeyPlay: {Key: "space"}, domain.KeyStop: {Key: "s"}},
	}, domain.Window{}, nil, nil)
	s.SetClips(domain.ClipList{{ID: 1, Name: "Intro", Timecode: "00:00:00:00", Duration: "00:00:10:00"}})
	return app.NewVirtualDeck(s, injector.NewMock())
}

func TestRespondPlayAck(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "play"})
	if !strings.HasPrefix(out, "200 ok") {
		t.Errorf("play response = %q", out)
	}
}

func TestRespondTransportInfo(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	_ = r.Handle(Command{Name: "play"})
	out := r.Handle(Command{Name: "transport info"})
	if !strings.HasPrefix(out, "208 transport info:") {
		t.Errorf("transport info head = %q", out)
	}
	if !strings.Contains(out, "status: play") {
		t.Errorf("transport info body missing status: %q", out)
	}
	if !strings.HasSuffix(out, "\r\n\r\n") {
		t.Errorf("multi-line response must end with blank line: %q", out)
	}
}

func TestRespondClips(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "clips get"})
	if !strings.HasPrefix(out, "205 clips info:") || !strings.Contains(out, "Intro") {
		t.Errorf("clips response = %q", out)
	}
}

func TestRespondUnknownCommand(t *testing.T) {
	d := newTestDeck()
	r := NewResponder(d, d)
	out := r.Handle(Command{Name: "frobnicate"})
	if !strings.HasPrefix(out, "100 ") {
		t.Errorf("unknown command should be 1xx: %q", out)
	}
}
