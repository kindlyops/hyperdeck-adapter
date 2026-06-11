package clipsource

import (
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// New builds the clip source named by the profile's clip_source.type.
func New(p domain.Profile) port.ClipSource {
	cfg := p.ClipSource
	switch cfg.Type {
	case "playlist_file":
		return NewPlaylist(cfg.Path)
	case "mitti":
		return NewMitti(defaultCount(cfg.Count))
	default: // "positional" and unknown -> positional
		return NewPositional(defaultCount(cfg.Count))
	}
}

func defaultCount(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}
