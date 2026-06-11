package hyperdeck

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/app"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestServerEndToEnd(t *testing.T) {
	mock := injector.NewMock()
	s := app.NewSession()
	s.Lock(domain.Profile{
		ID:        "vlc",
		Injection: domain.InjectionBackground,
		Keymap:    domain.Keymap{domain.KeyPlay: {Key: "space"}, domain.KeyStop: {Key: "s"}},
	}, domain.Window{Process: "vlc.exe"}, nil, nil)
	deck := app.NewVirtualDeck(s, mock)

	srv := NewServer(deck, deck)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(ln)
	defer ln.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	rd := bufio.NewReader(conn)

	// Server greets with a 500 connection info banner.
	banner, _ := rd.ReadString('\n')
	if !strings.HasPrefix(banner, "500 connection info:") {
		t.Errorf("banner = %q", banner)
	}
	drainBlank(rd)

	// Send "play" and expect "200 ok" + a recorded keystroke.
	conn.Write([]byte("play\r\n"))
	resp, _ := rd.ReadString('\n')
	if !strings.HasPrefix(resp, "200 ok") {
		t.Errorf("play resp = %q", resp)
	}
	// Allow the handler goroutine to record the keystroke.
	time.Sleep(50 * time.Millisecond)
	if len(mock.Sent) != 1 || mock.Sent[0].Chords[0].Key != "space" {
		t.Errorf("expected space keystroke, got %+v", mock.Sent)
	}
}

func drainBlank(rd *bufio.Reader) {
	for {
		line, err := rd.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			return
		}
	}
}
