// Package stateprobe implements port.StateProbe strategies.
package stateprobe

import (
	"regexp"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// TitleRegex infers playing state from the window title.
type TitleRegex struct{ re *regexp.Regexp }

// NewTitleRegex compiles pattern; an invalid pattern yields a probe that never detects.
func NewTitleRegex(pattern string) *TitleRegex {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &TitleRegex{re: nil}
	}
	return &TitleRegex{re: re}
}

// Detect reports playing when the title matches, stopped when it does not.
func (t *TitleRegex) Detect(w domain.Window) (domain.TransportState, bool) {
	if t.re == nil {
		return domain.StateStopped, false
	}
	if t.re.MatchString(w.Title) {
		return domain.StatePlaying, true
	}
	return domain.StateStopped, true
}
