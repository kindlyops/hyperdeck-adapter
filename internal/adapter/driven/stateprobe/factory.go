package stateprobe

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// New builds the state probe named by the profile's state.type. namer supplies
// UI Automation reads for the "uia" probe and may be nil when no uia profile is used.
func New(p domain.Profile, namer ElementNamer) port.StateProbe {
	switch p.State.Type {
	case "title_regex":
		return NewTitleRegex(p.State.Playing)
	case "uia":
		return NewUIA(namer, p.State.AutomationID, p.State.Playing)
	default:
		return None{}
	}
}
