package hyperdeck

import (
	"fmt"
	"strings"
)

// Command is a parsed HyperDeck request.
type Command struct {
	Name   string
	Params map[string]string
}

// ParseCommand parses one full command block (single- or multi-line).
func ParseCommand(raw string) (Command, error) {
	lines := splitLines(raw)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return Command{}, fmt.Errorf("empty command")
	}
	cmd := Command{Params: map[string]string{}}
	first := strings.TrimSpace(lines[0])

	// Inline form: "goto: clip id: 3" -> name "goto", param "clip id"="3".
	if name, rest, ok := strings.Cut(first, ":"); ok {
		cmd.Name = strings.TrimSpace(name)
		rest = strings.TrimSpace(rest)
		if rest != "" {
			if k, v, ok := strings.Cut(rest, ":"); ok {
				cmd.Params[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	} else {
		cmd.Name = first
	}

	// Block form: subsequent "key: value" lines until a blank line.
	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			break
		}
		if k, v, ok := strings.Cut(line, ":"); ok {
			cmd.Params[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return cmd, nil
}

func splitLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	return strings.Split(raw, "\n")
}
