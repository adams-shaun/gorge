package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
)

// TestCapabilitiesReportEvents proves GET /api/capabilities answers 200 with
// or without an events backend, and that its answer tracks whether the
// /api/events routes are mounted: the lobby's Draft strip rides on it.
func TestCapabilitiesReportEvents(t *testing.T) {
	cases := []struct {
		name       string
		opts       Options
		wantBody   string
		wantEvents int
	}{
		{"off", Options{}, `{"events":false}`, http.StatusNotFound},
		{"on", Options{Events: stubEvents{}}, `{"events":true}`, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { r.Close() })
			srv := httptest.NewServer(NewHandler(r, c.opts))
			t.Cleanup(srv.Close)

			resp, err := http.Get(srv.URL + "/api/capabilities")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /api/capabilities = %d, want 200: %s", resp.StatusCode, body)
			}
			if got := strings.TrimSpace(string(body)); got != c.wantBody {
				t.Errorf("GET /api/capabilities = %s, want %s", got, c.wantBody)
			}

			post, err := http.Post(srv.URL+"/api/capabilities", "application/json", strings.NewReader("{}"))
			if err != nil {
				t.Fatal(err)
			}
			defer post.Body.Close()
			if post.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("POST /api/capabilities = %d, want 405", post.StatusCode)
			}

			events, err := http.Get(srv.URL + "/api/events")
			if err != nil {
				t.Fatal(err)
			}
			defer events.Body.Close()
			if events.StatusCode != c.wantEvents {
				t.Errorf("GET /api/events = %d, want %d", events.StatusCode, c.wantEvents)
			}
		})
	}
}
