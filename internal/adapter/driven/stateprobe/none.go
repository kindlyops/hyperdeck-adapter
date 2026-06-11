package stateprobe

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// None performs no detection; the modeled state is authoritative.
type None struct{}

// Detect always reports not-detected.
func (None) Detect(domain.Window) (domain.TransportState, bool) {
	return domain.StateStopped, false
}
