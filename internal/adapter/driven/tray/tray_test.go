package tray

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestStatusText(t *testing.T) {
	if got := statusText(domain.LockState{Locked: false}); got != "Disconnected — no player" {
		t.Errorf("unlocked text = %q", got)
	}
	p := domain.Profile{ID: "vlc"}
	got := statusText(domain.LockState{Locked: true, Profile: &p, Window: domain.Window{Title: "Movie - VLC"}})
	if got == "" || got == "Disconnected — no player" {
		t.Errorf("locked text = %q", got)
	}
}
