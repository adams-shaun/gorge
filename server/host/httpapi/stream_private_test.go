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

// TestHelloIncludesOnlyTheSeatClaimedPrivateTable pins both halves of the
// private-table hello rule: a stream with no valid seat token sees only the
// lobby, while a stream that proves a claim to a private on-demand table also
// receives that table's TableInfo. The seated client needs the latter for
// auto-mana and restart; every other client must not learn the id.
func TestHelloIncludesOnlyTheSeatClaimedPrivateTable(t *testing.T) {
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
	if err := r.AddTable(host.TableConfig{ID: "g2", Name: "Other private game", Seats: 2,
		Decks: []string{"a", "b"}, Seed: 9, Spectator: view.Public, OnDemand: true}); err != nil {
		t.Fatal(err)
	}

	_, mux := newHandler(r, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		switch req.URL.Query().Get("token") {
		case "seat-token":
			return SeatClaim{Table: "g1", Seat: 0}, true
		case "other-token":
			return SeatClaim{Table: "g2", Seat: 0}, true
		}
		return SeatClaim{}, false
	}})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	readHello := func(url string) protocol.Hello {
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
		return h
	}
	has := func(h protocol.Hello, id string) bool {
		for _, ti := range h.Tables {
			if ti.ID == id {
				return true
			}
		}
		return false
	}

	// Precondition: both private tables exist and are addressable, so their
	// absence below is the hello filter and not a missing fixture.
	if got := r.Tables(); len(got) != 3 {
		t.Fatalf("registry has %d tables, want the public t1 and private g1, g2", len(got))
	}

	anon := readHello(srv.URL + "/api/stream")
	if has(anon, "g1") || has(anon, "g2") {
		t.Fatalf("anonymous hello advertised a private table: %+v", anon.Tables)
	}
	if !has(anon, "t1") {
		t.Fatalf("anonymous hello lost the lobby table: %+v", anon.Tables)
	}

	seated := readHello(srv.URL + "/api/stream?token=seat-token")
	if !has(seated, "g1") {
		t.Fatalf("seat-claimed hello omitted private table g1: %+v", seated.Tables)
	}
	if !has(seated, "t1") {
		t.Fatalf("seat-claimed hello lost the lobby table: %+v", seated.Tables)
	}
	if has(seated, "g2") {
		t.Fatalf("a claim for g1 also advertised g2: %+v", seated.Tables)
	}

	// A valid token for a different private table must reveal only that
	// table, never g1.
	other := readHello(srv.URL + "/api/stream?token=other-token")
	if !has(other, "g2") {
		t.Fatalf("g2-claimed hello omitted its own private table: %+v", other.Tables)
	}
	if has(other, "g1") {
		t.Fatalf("a claim for g2 advertised private table g1: %+v", other.Tables)
	}

	wrong := readHello(srv.URL + "/api/stream?token=wrong-token")
	if has(wrong, "g1") || has(wrong, "g2") {
		t.Fatalf("an invalid token advertised a private table: %+v", wrong.Tables)
	}
}
