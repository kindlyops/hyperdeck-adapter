package port

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// KeyInjector is a driven (outbound) port: deliver keystrokes to a window.
type KeyInjector interface {
	Focus(w domain.Window) error
	SendKeys(w domain.Window, chords []domain.Chord) error
}

// WindowEnumerator is a driven port: list currently-open OS windows.
type WindowEnumerator interface {
	OpenWindows() ([]domain.Window, error)
}

// ClipSource is a driven port: produce the active clip list.
type ClipSource interface {
	List() (domain.ClipList, error)
}

// StateProbe is a driven port: best-effort real-state detection.
type StateProbe interface {
	Detect(w domain.Window) (domain.TransportState, bool)
}

// StatusPresenter is a driven port: reflect lock status in the UI.
type StatusPresenter interface {
	Present(lock domain.LockState)
}

// ProfileStore is a driven port: load validated profiles.
type ProfileStore interface {
	Load() ([]domain.Profile, error)
}

// Clock is a driven port: a tick source the reconciler/locator poll on.
type Clock interface {
	Tick(d time.Duration) <-chan time.Time
}
