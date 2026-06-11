package stateprobe

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// New builds the state probe named by the profile's state.type.
func New(p domain.Profile) port.StateProbe {
	if p.State.Type == "title_regex" {
		return NewTitleRegex(p.State.Playing)
	}
	return None{}
}
