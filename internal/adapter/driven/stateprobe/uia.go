package stateprobe

import (
	"regexp"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// ElementNamer reads the Name of a UI Automation element (by AutomationId) on a
// window (by HWND). Implemented by the uia engine; nil/error reads are treated as
// "not detectable".
type ElementNamer interface {
	Name(hwnd uintptr, automationID string) (string, error)
}

// UIA infers playing state from a UI Automation element's Name — e.g. Example Player's
// TogglePlaybackButton is named "Pause" while playing and "Play" while paused.
type UIA struct {
	namer ElementNamer
	aid   string
	re    *regexp.Regexp
}

// NewUIA builds a probe that reads automationID's Name and reports playing when it
// matches playingPattern. An invalid pattern or nil namer yields a probe that
// never detects.
func NewUIA(namer ElementNamer, automationID, playingPattern string) *UIA {
	re, err := regexp.Compile(playingPattern)
	if err != nil {
		re = nil
	}
	return &UIA{namer: namer, aid: automationID, re: re}
}

// Detect reads the element Name and reports playing when it matches. A read that
// fails or finds nothing (controls hidden, no clip open) reports not-detected, so
// the modeled state is left untouched.
func (u *UIA) Detect(w domain.Window) (domain.TransportState, bool) {
	if u.namer == nil || u.re == nil {
		return domain.StateStopped, false
	}
	name, err := u.namer.Name(w.Handle, u.aid)
	if err != nil || name == "" {
		return domain.StateStopped, false
	}
	if u.re.MatchString(name) {
		return domain.StatePlaying, true
	}
	return domain.StateStopped, true
}
