package clipsource

import (
	"fmt"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Positional produces a fixed number of generic clip slots.
type Positional struct{ count int }

// NewPositional returns a positional clip source with n slots.
func NewPositional(n int) *Positional { return &Positional{count: n} }

// List returns n generically-named clips.
func (p *Positional) List() (domain.ClipList, error) {
	clips := make(domain.ClipList, 0, p.count)
	for i := 1; i <= p.count; i++ {
		clips = append(clips, domain.Clip{ID: i, Name: fmt.Sprintf("Clip %d", i)})
	}
	return clips, nil
}
