package tray

import "testing"

func TestCheckedProfile(t *testing.T) {
	profiles := []string{"vlc", "mitti"}
	cases := []struct {
		name   string
		active string
		want   string
	}{
		{"auto when empty", "", ""},
		{"known id checks itself", "mitti", "mitti"},
		{"unknown id falls back to auto", "ghost", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkedProfile(profiles, c.active); got != c.want {
				t.Errorf("checkedProfile(%v, %q) = %q want %q", profiles, c.active, got, c.want)
			}
		})
	}
}
