package app

import (
	"sync"
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

	mu     sync.Mutex
	active string // pinned profile id; "" means Auto / match any
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
	return &LockManager{
		session:   s,
		windows:   w,
		profiles:  profiles,
		presenter: presenter,
		clipsFor:  clipsFor,
		probeFor:  probeFor,
	}
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

// SetActive pins the matcher to one profile id; "" restores Auto (match any).
// It re-polls immediately so the lock state reflects the new selection.
func (lm *LockManager) SetActive(id string) {
	lm.mu.Lock()
	lm.active = id
	lm.mu.Unlock()
	lm.Poll()
}

// Run polls on every clock tick until the channel closes.
func (lm *LockManager) Run(clock port.Clock, every time.Duration) {
	for range clock.Tick(every) {
		lm.Poll()
	}
}

func (lm *LockManager) firstMatch(windows []domain.Window) (domain.Profile, domain.Window, bool) {
	lm.mu.Lock()
	active := lm.active
	lm.mu.Unlock()
	for _, p := range lm.profiles {
		if active != "" && p.ID != active {
			continue
		}
		for _, w := range windows {
			if p.MatchesWindow(w) {
				return p, w, true
			}
		}
	}
	return domain.Profile{}, domain.Window{}, false
}
