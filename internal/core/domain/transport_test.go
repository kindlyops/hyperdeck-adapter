package domain

import "testing"

func TestTransportStateHyperDeckStatus(t *testing.T) {
	if StateStopped.HyperDeckStatus() != "stopped" {
		t.Errorf("StateStopped = %q", StateStopped.HyperDeckStatus())
	}
	if StatePlaying.HyperDeckStatus() != "play" {
		t.Errorf("StatePlaying = %q", StatePlaying.HyperDeckStatus())
	}
}

func TestClipListIndexing(t *testing.T) {
	cl := ClipList{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
	if cl.Len() != 2 {
		t.Fatalf("Len = %d", cl.Len())
	}
}
