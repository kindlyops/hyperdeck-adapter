// Package clock implements port.Clock over the system clock.
package clock

import (
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Clock ticks on real time.
type Clock struct{}

// New returns a system clock.
func New() *Clock { return &Clock{} }

// Tick returns a channel that fires every d.
func (c *Clock) Tick(d time.Duration) <-chan time.Time {
	return time.NewTicker(d).C
}

var _ port.Clock = (*Clock)(nil)
