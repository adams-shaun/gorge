package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// TestUndoEndpoint uses the same seat-claim fence as intent submission and
// keeps the match number unchanged while the play loop rewinds in place.
func TestUndoEndpoint(t *testing.T) {
	r, err := host.New(host.Options{LoadDeck: loader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "undo", Seats: 2, Decks: []string{"a", "b"},
		Seed: 5, Spectator: view.Omniscient, Humans: []int{0}}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	first := waitHTTPPending(t, r, 0, 0)
	if err := r.SubmitIntent("t1", 1, 0, answerFor(first)); err != nil {
		t.Fatal(err)
	}
	_ = waitHTTPPending(t, r, 0, first.Seq)

	srv := httptest.NewServer(NewHandler(r, Options{Seat: claimResolver(map[string]state.PlayerID{"s0": 0})}))
	t.Cleanup(srv.Close)
	code, _, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/undo", "s0", nil)
	if code != http.StatusNoContent {
		t.Fatalf("undo status %d, want 204", code)
	}
	rewound := waitHTTPPendingSeq(t, r, 0, first.Seq)
	if rewound.Seq != first.Seq {
		t.Fatalf("rewound decision seq %d, want first answered seq %d", rewound.Seq, first.Seq)
	}
	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].Match != 1 {
		t.Fatalf("in-place undo changed match identity: %+v, %v", ms, err)
	}
}

// TestUndoEndpointRejectsMultiHumanTable exposes the consent-flow boundary as
// a clear 409 rather than accepting a whole-table rollback from one player.
func TestUndoEndpointRejectsMultiHumanTable(t *testing.T) {
	srv, r := parkedSeatServer(t, map[string]state.PlayerID{"s0": 0, "s1": 1})
	parked, _ := parkedSeat(t, r)
	session := "s0"
	if parked == 1 {
		session = "s1"
	}
	code, body, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/undo", session, nil)
	if code != http.StatusConflict || !stringContainsHTTP(body.Message, "human seats") {
		t.Fatalf("multi-human undo: status %d body %+v", code, body)
	}
}

// TestUndoEndpointRejectsAfterMatchEnd observes the terminal stream frame
// first, then proves a later POST cannot receive the 204 reserved for a queued
// rewind. The human seat's short think budget lets its deterministic caretaker
// finish the compact seed-55 fixture without an HTTP driver.
func TestUndoEndpointRejectsAfterMatchEnd(t *testing.T) {
	r, err := host.New(host.Options{
		LoadDeck:     loader(t),
		Sleep:        func(time.Duration, <-chan struct{}) {},
		ThinkTimeout: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "finished undo", Seats: 2, Decks: []string{"a", "b"},
		Seed: 55, Spectator: view.Omniscient, Humans: []int{0}}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}

	s := r.OpenSession()
	t.Cleanup(func() { r.CloseSession(s.ID) })
	if err := r.Subscribe(s, "t1", protocol.ModeOverview); err != nil {
		t.Fatal(err)
	}
	matchEnd := make(chan struct{})
	go func() {
		for f := range s.Out() {
			if f.T == protocol.TMatchEnd {
				close(matchEnd)
				return
			}
		}
	}()
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-matchEnd:
	case <-time.After(20 * time.Second):
		t.Fatal("stream did not emit match_end")
	}

	srv := httptest.NewServer(NewHandler(r, Options{Seat: claimResolver(map[string]state.PlayerID{"s0": 0})}))
	t.Cleanup(srv.Close)
	code, body, _ := seatReq(t, http.MethodPost, srv.URL+"/api/tables/t1/matches/1/undo", "s0", nil)
	if code != http.StatusConflict || body.Code != "conflict" {
		t.Fatalf("undo after match_end: status %d body %+v, want 409 conflict", code, body)
	}
}

func waitHTTPPending(t *testing.T, r *host.Registry, p state.PlayerID, notSeq uint64) *decision.Decision {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if d, err := r.Pending("t1", 1, p); err == nil && d.Seq != notSeq {
			return d
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("seat %d did not receive a pending decision", p)
	return nil
}

func waitHTTPPendingSeq(t *testing.T, r *host.Registry, p state.PlayerID, want uint64) *decision.Decision {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if d, err := r.Pending("t1", 1, p); err == nil && d.Seq == want {
			return d
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("seat %d did not return to decision seq %d", p, want)
	return nil
}

func stringContainsHTTP(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
