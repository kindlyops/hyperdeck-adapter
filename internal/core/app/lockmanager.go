package app

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// ClipSourceFactory builds a clip source for a profile (provided by the composition root).
type ClipSourceFactory func(domain.Profile) port.ClipSource

// StateProbeFactory builds a state probe for a profile.
type StateProbeFactory func(domain.Profile) port.StateProbe

// LockManager binds a running player to the session by matching profiles.
type LockManager struct {
	session   *Session
	windows   port.WindowEnumerator
	profiles  []domain.Profile
	presenter port.StatusPresenter
	clipsFor  ClipSourceFactory
	probeFor  StateProbeFactory
}

// NewLockManager wires a lock manager.
func NewLockManager(
	s *Session,
	w port.WindowEnumerator,
	profiles []domain.Profile,
	presenter port.StatusPresenter,
	clipsFor ClipSourceFactory,
	probeFor StateProbeFactory,
) *LockManager {
	return &LockManager{s, w, profiles, presenter, clipsFor, probeFor}
}

// Poll runs one match cycle: lock on first match, unlock when the locked window is gone.
func (lm *LockManager) Poll() {
	windows, err := lm.windows.OpenWindows()
	if err != nil {
		windows = nil
	}
	if profile, win, ok := lm.firstMatch(windows); ok {
		if cur, _, locked := lm.session.Active(); !locked || cur.ID != profile.ID {
			lm.session.Lock(profile, win, lm.clipsFor(profile), lm.probeFor(profile))
			lm.presenter.Present(lm.session.LockState())
		}
		return
	}
	if _, _, locked := lm.session.Active(); locked {
		lm.session.Unlock()
		lm.presenter.Present(lm.session.LockState())
	}
}

// Run polls on every clock tick until the channel closes.
func (lm *LockManager) Run(clock port.Clock, every time.Duration) {
	for range clock.Tick(every) {
		lm.Poll()
	}
}

func (lm *LockManager) firstMatch(windows []domain.Window) (domain.Profile, domain.Window, bool) {
	for _, p := range lm.profiles {
		for _, w := range windows {
			if p.MatchesWindow(w) {
				return p, w, true
			}
		}
	}
	return domain.Profile{}, domain.Window{}, false
}
