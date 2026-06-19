//go:build !windows

package uia

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// Engine is a stub on non-Windows platforms; UI Automation is Windows-only.
type Engine struct{}

// New returns the stub engine.
func New() *Engine { return &Engine{} }

// Control reports that UIA control is unavailable off Windows.
func (e *Engine) Control(domain.Profile, domain.Window, domain.KeyName) error {
	return fmt.Errorf("uia control is only supported on Windows")
}

// Name reports that UIA reads are unavailable off Windows.
func (e *Engine) Name(uintptr, string) (string, error) {
	return "", fmt.Errorf("uia is only supported on Windows")
}

var _ port.PlayerController = (*Engine)(nil)
