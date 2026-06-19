// Package vlchttp implements port.PlayerController by driving VLC through its
// HTTP "requests" interface (Preferences > Interface > Main interfaces > Web,
// or `vlc --extraintf http --http-password <pw>`). It is the control backend for
// ControlAPI profiles whose api.type is "vlc_http".
//
// VLC on Windows ignores most synthesized hotkeys (only play/pause registers
// reliably), so its transport is driven out-of-band via this API instead of the
// keystroke injector.
package vlchttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/port"
)

// DefaultBaseURL is used when a profile's api.base_url is empty.
const DefaultBaseURL = "http://127.0.0.1:8080"

// commandFor maps a logical transport action to a VLC `requests/status.json`
// command. Actions with no VLC equivalent (e.g. record) are absent and treated
// as an acked no-op, mirroring the injector's handling of unmapped keys.
var commandFor = map[domain.KeyName]string{
	domain.KeyPlay: "pl_play",     // start playback of the current item
	domain.KeyStop: "pl_stop",     // stop playback
	domain.KeyNext: "pl_next",     // next playlist item
	domain.KeyPrev: "pl_previous", // previous playlist item
}

// Controller drives VLC over HTTP. The zero value is not usable; call New.
type Controller struct {
	client *http.Client
}

// New returns a VLC HTTP controller with a bounded per-request timeout.
func New() *Controller {
	return &Controller{client: &http.Client{Timeout: 4 * time.Second}}
}

// Control issues the VLC command for key against the player described by the
// profile's api config. The window is unused (VLC is addressed by URL, not HWND).
func (c *Controller) Control(p domain.Profile, _ domain.Window, key domain.KeyName) error {
	cmd, ok := commandFor[key]
	if !ok {
		return nil // no VLC equivalent: acked no-op
	}

	base := p.API.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	endpoint, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("vlc control %q: invalid base url %q: %w", key, base, err)
	}
	endpoint.Path = "/requests/status.json"
	endpoint.RawQuery = url.Values{"command": {cmd}}.Encode()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("vlc control %q: build request: %w", key, err)
	}
	// VLC's HTTP interface uses Basic auth with an empty username.
	req.SetBasicAuth("", p.API.Password)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("vlc control %q (%s): %w", key, cmd, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain so the connection can be reused

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("vlc control %q: unauthorized — check api.password matches VLC's HTTP password", key)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("vlc control %q (%s): HTTP %d", key, cmd, resp.StatusCode)
	}
	return nil
}

var _ port.PlayerController = (*Controller)(nil)
