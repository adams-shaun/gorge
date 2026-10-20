package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestFocusSubscribeToPrivateTableRequiresSeatClaim closes the subscribe
// path: ids are sequential (g1, g2, ...), so POST /api/subscribe must not
// admit an anonymous session to a private on-demand table. The claim the
// stream already resolved is the authority; a session without one gets the
// same 404 as an unknown table, while a session whose claim names the table
// still subscribes.
func TestFocusSubscribeToPrivateTableRequiresSeatClaim(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: loader(t), Ring: 16, Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "Lobby", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 7, Spectator: view.Public}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddTable(host.TableConfig{ID: "g1", Name: "Private game", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 8, Spectator: view.Public, OnDemand: true}); err != nil {
		t.Fatal(err)
	}
	// Precondition: the private table exists, so a 404 below is the claim
	// fence and not a missing fixture.
	if got := r.Tables(); len(got) != 2 {
		t.Fatalf("registry has %d tables, want the public t1 and private g1", len(got))
	}

	_, mux := newHandler(r, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		if req.URL.Query().Get("token") == "seat-token" {
			return SeatClaim{Table: "g1", Seat: 0}, true
		}
		return SeatClaim{}, false
	}})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	sessionID := func(url string) string {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		frames := readSSE(t, resp.Body, 1)
		if len(frames) != 1 || frames[0].event != "hello" {
			t.Fatalf("%s: first frame %+v, want hello", url, frames)
		}
		var h protocol.Hello
		if err := frames[0].frame.Decode(&h); err != nil {
			t.Fatal(err)
		}
		return h.Session
	}

	anon := sessionID(srv.URL + "/api/stream")
	for _, mode := range []string{protocol.ModeFocus, protocol.ModeOverview} {
		code, body := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: anon, Table: "g1", Mode: mode})
		if code != http.StatusNotFound || body.Code != "not_found" {
			t.Fatalf("anonymous %s subscribe to g1 = %d %+v, want 404 not_found", mode, code, body)
		}
	}
	// The same anonymous session may still watch the public table.
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: anon, Table: "t1", Mode: protocol.ModeFocus}); code != http.StatusNoContent {
		t.Fatalf("anonymous focus subscribe to public t1 = %d, want 204", code)
	}

	seated := sessionID(srv.URL + "/api/stream?token=seat-token")
	if code, _ := postJSON(t, srv.URL+"/api/subscribe", protocol.Subscribe{Session: seated, Table: "g1", Mode: protocol.ModeFocus}); code != http.StatusNoContent {
		t.Fatalf("seat-claimed focus subscribe to g1 = %d, want 204", code)
	}
}
