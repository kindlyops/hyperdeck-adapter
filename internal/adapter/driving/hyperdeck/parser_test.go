package hyperdeck

import (
	"reflect"
	"testing"
)

func TestParseSimpleCommand(t *testing.T) {
	cmd, err := ParseCommand("play\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "play" || len(cmd.Params) != 0 {
		t.Errorf("got %+v", cmd)
	}
}

func TestParseCommandWithParams(t *testing.T) {
	raw := "play:\r\nsingle clip: true\r\nspeed: 100\r\n\r\n"
	cmd, err := ParseCommand(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"single clip": "true", "speed": "100"}
	if cmd.Name != "play" || !reflect.DeepEqual(cmd.Params, want) {
		t.Errorf("got %+v", cmd)
	}
}

func TestParseGoto(t *testing.T) {
	cmd, err := ParseCommand("goto: clip id: 3\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "goto" || cmd.Params["clip id"] != "3" {
		t.Errorf("got %+v", cmd)
	}
}
