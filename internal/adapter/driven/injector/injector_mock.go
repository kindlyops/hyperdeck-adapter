package injector

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// SentKeys records one SendKeys call.
type SentKeys struct {
	Window domain.Window
	Chords []domain.Chord
}

// Mock is an in-memory KeyInjector + WindowEnumerator for tests.
type Mock struct {
	Windows  []domain.Window // returned by OpenWindows
	Focused  []domain.Window
	Sent     []SentKeys
	FocusErr error
	SendErr  error
	EnumErr  error
}

// NewMock returns an empty Mock.
func NewMock() *Mock { return &Mock{} }

func (m *Mock) Focus(w domain.Window) error {
	if m.FocusErr != nil {
		return m.FocusErr
	}
	m.Focused = append(m.Focused, w)
	return nil
}

func (m *Mock) SendKeys(w domain.Window, chords []domain.Chord) error {
	if m.SendErr != nil {
		return m.SendErr
	}
	m.Sent = append(m.Sent, SentKeys{Window: w, Chords: chords})
	return nil
}

func (m *Mock) OpenWindows() ([]domain.Window, error) {
	if m.EnumErr != nil {
		return nil, m.EnumErr
	}
	return m.Windows, nil
}
