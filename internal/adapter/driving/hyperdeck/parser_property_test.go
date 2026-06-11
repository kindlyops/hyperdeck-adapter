package hyperdeck

import (
	"testing"

	"pgregory.net/rapid"
)

func TestParseNeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "raw")
		_, _ = ParseCommand(s) // must never panic on arbitrary input
	})
}
