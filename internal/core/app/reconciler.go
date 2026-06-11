package app

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Reconciler refreshes the clip list and corrects modeled state from the probe.
type Reconciler struct {
	session *Session
}

// NewReconciler wires a reconciler to the session.
func NewReconciler(s *Session) *Reconciler {
	return &Reconciler{session: s}
}

// Tick runs one reconciliation cycle. Safe to call when unlocked.
func (r *Reconciler) Tick() {
	_, w, ok := r.session.Active()
	if !ok {
		return
	}
	if cs := r.session.ClipSource(); cs != nil {
		if clips, err := cs.List(); err == nil {
			r.session.SetClips(clips)
		}
	}
	if probe := r.session.Probe(); probe != nil {
		if state, detected := probe.Detect(w); detected {
			r.session.SetState(state)
		}
	}
}

// Run reconciles on every clock tick until the channel closes.
func (r *Reconciler) Run(clock port.Clock, every time.Duration) {
	for range clock.Tick(every) {
		r.Tick()
	}
}
