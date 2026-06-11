package clock

import (
	"testing"
	"time"
)

func TestTickFires(t *testing.T) {
	c := New()
	ch := c.Tick(10 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected a tick within 1s")
	}
}
