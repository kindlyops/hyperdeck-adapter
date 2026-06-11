package app

import (
	"sync"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Session is the mutex-guarded shared state of the running deck.
type Session struct {
	mu          sync.Mutex
	lock        domain.LockState
	state       domain.TransportState
	clips       domain.ClipList
	currentClip int
	clipSource  port.ClipSource
	probe       port.StateProbe
}

// NewSession returns an unlocked session in the stopped state.
func NewSession() *Session {
	return &Session{currentClip: 1}
}

// Lock binds an active profile, window, clip source, and state probe.
func (s *Session) Lock(p domain.Profile, w domain.Window, cs port.ClipSource, sp port.StateProbe) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prof := p
	s.lock = domain.LockState{Locked: true, Profile: &prof, Window: w}
	s.clipSource = cs
	s.probe = sp
	s.state = domain.StateStopped
	s.currentClip = 1
}

// Unlock clears the active binding.
func (s *Session) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lock = domain.LockState{}
	s.clipSource = nil
	s.probe = nil
	s.clips = nil
}

// Active returns the locked profile and window, or ok=false when unlocked.
func (s *Session) Active() (domain.Profile, domain.Window, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lock.Locked || s.lock.Profile == nil {
		return domain.Profile{}, domain.Window{}, false
	}
	return *s.lock.Profile, s.lock.Window, true
}

// LockState returns a copy of the current lock status.
func (s *Session) LockState() domain.LockState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock
}

func (s *Session) State() domain.TransportState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Session) SetState(st domain.TransportState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = st
}

func (s *Session) Clips() domain.ClipList {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clips
}

func (s *Session) SetClips(c domain.ClipList) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clips = c
}

func (s *Session) CurrentClip() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentClip
}

func (s *Session) SetCurrentClip(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentClip = n
}

// ClipSource returns the active clip source (nil when unlocked).
func (s *Session) ClipSource() port.ClipSource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clipSource
}

// Probe returns the active state probe (nil when unlocked).
func (s *Session) Probe() port.StateProbe {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.probe
}
