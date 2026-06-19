//go:build !windows

package uia

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Controller is a stub on non-Windows platforms; UI Automation is Windows-only.
type Controller struct{}

// New returns the stub controller.
func New() *Controller { return &Controller{} }

// Control reports that UIA control is unavailable off Windows.
func (c *Controller) Control(domain.Profile, domain.Window, domain.KeyName) error {
	return fmt.Errorf("uia control is only supported on Windows")
}

var _ port.PlayerController = (*Controller)(nil)
