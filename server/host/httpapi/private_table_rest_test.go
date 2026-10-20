package httpapi

// A private play-vs-bot table (OnDemand && Event == "") is discoverable only
// through its join URL: the lobby and the stream fence it behind a seat claim.
// These tests pin the same fence on the table-scoped REST reads, so a guessed
// sequential gN id answers exactly the unknown-id not-found.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// privateRestServer serves one private play-vs-bot table (g1) and one public
// event table (e1), both finished, so every read route has a match to serve.
// The claim resolver names the table each token holds, so a g1 request can
// present either the right claim or a public-table claim.
func privateRestServer(t *testing.T) (*httptest.Server, *host.Registry) {
	t.Helper()
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	for _, cfg := range []host.TableConfig{
		{ID: "g1", Name: "Play vs bot", Seats: 2, Decks: []string{"a", "b"}, Seed: 2,
			Spectator: view.Omniscient, OnDemand: true},
		{ID: "e1", Name: "Event table", Seats: 2, Decks: []string{"a", "b"}, Seed: 3,
			Spectator: view.Omniscient, OnDemand: true, Event: "evt"},
	} {
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []host.TableID{"g1", "e1"} {
		if err := r.Start(id); err != nil {
			t.Fatal(err)
		}
		r.Wait(id)
		ms, err := r.Matches(id)
		if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
			t.Fatalf("%s: match did not finish: err=%v ms=%+v", id, err, ms)
		}
	}
	srv := httptest.NewServer(NewHandler(r, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		switch req.Header.Get("X-Seat-Session") {
		case "g1-seat0":
			return SeatClaim{Table: "g1", Seat: 0}, true
		case "e1-seat0":
			return SeatClaim{Table: "e1", Seat: 0}, true
		default:
			return SeatClaim{}, false
		}
	}}))
	t.Cleanup(srv.Close)
	return srv, r
}

func TestPrivatePlayVsBotTableRestReadsNeedSeatClaim(t *testing.T) {
	srv, _ := privateRestServer(t)
	routes := []struct {
		name string
		path string
	}{
		{"matches", "/api/tables/%s/matches"},
		{"view", "/api/tables/%s/matches/1/view"},
		{"events", "/api/tables/%s/matches/1/events"},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			unknown := fmt.Sprintf(route.path, "zz9")
			code, unknownErr := getJSON(t, srv.URL+unknown, nil)
			if code != http.StatusNotFound || unknownErr.Code != "not_found" {
				t.Fatalf("unknown id: %d %+v", code, unknownErr)
			}

			private := fmt.Sprintf(route.path, "g1")
			code, privateErr := getJSON(t, srv.URL+private, nil)
			if code != http.StatusNotFound || privateErr != unknownErr {
				t.Fatalf("unclaimed private table: %d %+v, want the unknown-id not-found %+v", code, privateErr, unknownErr)
			}

			// A seat query must not become an existence oracle: an unclaimed
			// ?seat=0 (401) and a malformed ?seat=abc (400) answer exactly as
			// they do for an unknown id, never a private-only 404.
			for _, q := range []struct{ name, query string }{
				{"unclaimed-seat", "?seat=0"},
				{"malformed-seat", "?seat=abc"},
			} {
				t.Run(q.name, func(t *testing.T) {
					unknownCode, unknownBody := getJSON(t, srv.URL+unknown+q.query, nil)
					privateCode, privateBody := getJSON(t, srv.URL+private+q.query, nil)
					if privateCode != unknownCode || privateBody != unknownBody {
						t.Fatalf("private table %s: %d %+v, want unknown-id %d %+v", q.query, privateCode, privateBody, unknownCode, unknownBody)
					}
				})
			}

			code, wrongErr, _ := seatReq(t, http.MethodGet, srv.URL+private, "e1-seat0", nil)
			if code != http.StatusNotFound || wrongErr != unknownErr {
				t.Fatalf("private table with another table's claim: %d %+v", code, wrongErr)
			}

			code, _, raw := seatReq(t, http.MethodGet, srv.URL+private, "g1-seat0", nil)
			if code != http.StatusOK || len(raw) == 0 {
				t.Fatalf("private table with its own claim: %d %s", code, raw)
			}
			if code, _, raw := seatReq(t, http.MethodGet, srv.URL+private+"?seat=0", "g1-seat0", nil); code != http.StatusOK || len(raw) == 0 {
				t.Fatalf("private table with its own claim and ?seat=0: %d %s", code, raw)
			}

			publicEvent := fmt.Sprintf(route.path, "e1")
			if code, e := getJSON(t, srv.URL+publicEvent, nil); code != http.StatusOK {
				t.Fatalf("public event table without a claim: %d %+v", code, e)
			}
			if code, e, _ := seatReq(t, http.MethodGet, srv.URL+publicEvent, "e1-seat0", nil); code != http.StatusOK {
				t.Fatalf("public event table with a claim: %d %+v", code, e)
			}
		})
	}
}
