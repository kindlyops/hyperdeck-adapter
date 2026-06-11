package injector

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestMockRecordsCalls(t *testing.T) {
	m := NewMock()
	w := domain.Window{Process: "vlc.exe"}
	if err := m.Focus(w); err != nil {
		t.Fatal(err)
	}
	if err := m.SendKeys(w, []domain.Chord{{Key: "space"}}); err != nil {
		t.Fatal(err)
	}
	if len(m.Focused) != 1 || m.Focused[0] != w {
		t.Errorf("Focused = %+v", m.Focused)
	}
	if len(m.Sent) != 1 || m.Sent[0].Chords[0].Key != "space" {
		t.Errorf("Sent = %+v", m.Sent)
	}
}
