package vlchttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func apiProfile(baseURL string) domain.Profile {
	return domain.Profile{
		ID:      "vlc",
		Control: domain.ControlAPI,
		API:     domain.APIConfig{Type: "vlc_http", BaseURL: baseURL, Password: "pw"},
	}
}

func TestControlIssuesMappedCommands(t *testing.T) {
	cases := map[domain.KeyName]string{
		domain.KeyPlay: "pl_play",
		domain.KeyStop: "pl_stop",
		domain.KeyNext: "pl_next",
		domain.KeyPrev: "pl_previous",
	}
	for key, wantCmd := range cases {
		var gotPath, gotQuery, gotUser, gotPass string
		var authOK bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotQuery = r.URL.Query().Get("command")
			gotUser, gotPass, authOK = r.BasicAuth()
			w.WriteHeader(http.StatusOK)
		}))

		err := New().Control(apiProfile(srv.URL), domain.Window{}, key)
		srv.Close()

		if err != nil {
			t.Errorf("%s: unexpected error: %v", key, err)
		}
		if gotPath != "/requests/status.json" {
			t.Errorf("%s: path = %q, want /requests/status.json", key, gotPath)
		}
		if gotQuery != wantCmd {
			t.Errorf("%s: command = %q, want %q", key, gotQuery, wantCmd)
		}
		if !authOK || gotUser != "" || gotPass != "pw" {
			t.Errorf("%s: basic auth = (%q,%q,%v), want empty user + pw", key, gotUser, gotPass, authOK)
		}
	}
}

func TestControlUnmappedKeyIsNoOp(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer srv.Close()

	// Record has no VLC equivalent: should not hit the server and should ack.
	if err := New().Control(apiProfile(srv.URL), domain.Window{}, domain.KeyRecord); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("unmapped key should not issue an HTTP request")
	}
}

func TestControlUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := New().Control(apiProfile(srv.URL), domain.Window{}, domain.KeyPlay)
	if err == nil {
		t.Fatal("expected error on 401")
	}
}

func TestControlServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if err := New().Control(apiProfile(srv.URL), domain.Window{}, domain.KeyStop); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestControlDefaultsBaseURL(t *testing.T) {
	// With an empty base URL the controller falls back to DefaultBaseURL; the
	// request will fail (nothing listening), but it must not panic or misroute.
	p := apiProfile("")
	if p.API.BaseURL != "" {
		t.Fatal("precondition: base url should be empty")
	}
	_ = New().Control(p, domain.Window{}, domain.KeyPlay) // error expected, just no panic
}
