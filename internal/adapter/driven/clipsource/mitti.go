package clipsource

import "github.com/kindlyops/hyperdeck-adapter/internal/core/domain"

// Mitti is a best-effort clip source for Mitti. Until the proprietary playlist
// format is parsed, it falls back to a positional list of the configured size.
// Tracked as an open item in the design spec.
type Mitti struct{ fallback *Positional }

// NewMitti returns a Mitti clip source with a positional fallback of n slots.
func NewMitti(n int) *Mitti { return &Mitti{fallback: NewPositional(n)} }

// List returns the best-effort clip list.
func (m *Mitti) List() (domain.ClipList, error) { return m.fallback.List() }
