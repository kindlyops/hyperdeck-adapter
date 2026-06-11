// Package clipsource implements port.ClipSource strategies.
package clipsource

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

// Playlist reads clip names from an .m3u or .xspf file.
type Playlist struct{ path string }

// NewPlaylist returns a playlist-backed clip source.
func NewPlaylist(path string) *Playlist { return &Playlist{path: path} }

// List parses the playlist file into a clip list.
func (p *Playlist) List() (domain.ClipList, error) {
	switch strings.ToLower(filepath.Ext(p.path)) {
	case ".xspf":
		return p.listXSPF()
	default:
		return p.listM3U()
	}
}

func (p *Playlist) listM3U() (domain.ClipList, error) {
	f, err := os.Open(p.path)
	if err != nil {
		return nil, fmt.Errorf("open playlist %q: %w", p.path, err)
	}
	defer f.Close()

	var clips domain.ClipList
	name := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#EXTINF:"):
			if comma := strings.Index(line, ","); comma >= 0 {
				name = strings.TrimSpace(line[comma+1:])
			}
		case line == "" || strings.HasPrefix(line, "#"):
			// skip directives and blanks
		default:
			label := name
			if label == "" {
				label = filepath.Base(line)
			}
			clips = append(clips, domain.Clip{ID: len(clips) + 1, Name: label})
			name = ""
		}
	}
	return clips, sc.Err()
}

type xspfFile struct {
	Tracks []struct {
		Title    string `xml:"title"`
		Location string `xml:"location"`
	} `xml:"trackList>track"`
}

func (p *Playlist) listXSPF() (domain.ClipList, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("read playlist %q: %w", p.path, err)
	}
	var parsed xspfFile
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parse xspf %q: %w", p.path, err)
	}
	var clips domain.ClipList
	for _, tr := range parsed.Tracks {
		name := tr.Title
		if name == "" {
			name = filepath.Base(tr.Location)
		}
		clips = append(clips, domain.Clip{ID: len(clips) + 1, Name: name})
	}
	return clips, nil
}
