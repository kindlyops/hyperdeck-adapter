package clipsource

import (
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func TestPlaylistM3U(t *testing.T) {
	clips, err := NewPlaylist("../../../../testdata/sample.m3u").List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 2 || clips[0].Name != "Intro Clip" || clips[0].ID != 1 {
		t.Errorf("m3u clips = %+v", clips)
	}
}

func TestPlaylistXSPF(t *testing.T) {
	clips, err := NewPlaylist("../../../../testdata/sample.xspf").List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 2 || clips[1].Name != "Main Segment" {
		t.Errorf("xspf clips = %+v", clips)
	}
}

func TestPositional(t *testing.T) {
	clips, err := NewPositional(3).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 3 || clips[2].ID != 3 || clips[2].Name != "Clip 3" {
		t.Errorf("positional clips = %+v", clips)
	}
}

func TestFactory(t *testing.T) {
	p := domain.Profile{ClipSource: domain.ClipSourceConfig{Type: "positional", Count: 2}}
	cs := New(p)
	clips, _ := cs.List()
	if len(clips) != 2 {
		t.Errorf("factory positional = %+v", clips)
	}
}
