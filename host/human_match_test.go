package host

// Task M2b-5: a human seat plays a full bot-table match, replay-identically.
//
// M2b-1..4 each landed unit tests; nothing yet drives all of them together
// through one real game. This is that integration seam: a non-perpetual
// 4-seat table loaded from the repo decks, seat 0 a HumanSeat (M2b-2), the
// other three the ordinary bots M2b-1's defaultSeats builds, with the M2b-3
// think-timeout caretaker armed but -- because the human answers everything
// it is asked -- never allowed to fire. The embedder-facing loop reads the
// human seat through Pending and answers through SubmitIntent, exactly "the
// way an embedder would": a human and three bots all committed to the same
// intent stream, so the whole log is as if every seat had been a bot all
// along (D3).
//
// The three assertions are the three properties M2b-1..4 each assumed the
// others would guarantee:
//
//  1. the match reaches MatchFinished -- completion, not a crash, not a stall;
//  2. the caretaker never fires -- the human answers every decision it is
//     asked, so ThinkTimeout/ctx-cancel never substitute even one answer. We
//     add a per-seat counter as the cheapest honest signal; nothing else in
//     the package today distinguishes a caretaker's intent from a human's --
//     by design D3 they are byte-identical, which is exactly why a counter
//     is the one way to observe silence;
//  3. the logged game replays byte-identically -- replay.Replay over the
//     finished log reaches the same chain head, so a human-driven game is
//     indistinguishable from a recorded one.
//
// The answer builder is deterministic: the first Min options, always legal
// for any Decision.Kind. No math/rand, and no bot answering the human seat
// (that would be bots-agree-with-bots tautology). The 4-seat shape is
// deliberate: it is the integration seam, and two seats would miss the
// interleaving of human and bot turns across a whole table.

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// repoDeckLoader resolves every embedded repo deck through the compiled
// corpus once, up front, so the table plays the real Legacy archetypes and
// the loader is a pure function of the (deterministic) corpus. It mirrors
// sampleLoader's closed-over name-keyed map shape (host_test.go), so the
// host never reads files for decks itself.
func repoDeckLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	r := testutil.CorpusRegistry(t)
	byName := make(map[string][]*cards.Card, len(testutil.RepoDeckNames()))
	for _, n := range testutil.RepoDeckNames() {
		byName[n] = testutil.RepoDeck(t, r, n)
	}
	return func(name string) (Deck, error) {
		cs, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

// humanFirstSeat builds defaultSeats' bots and replaces slot 0 with a fresh
// HumanSeat, handing the caller back the *HumanSeat so the test can read its
// caretaker counter afterwards.
func humanFirstSeat(human **HumanSeat) func(names []string, seed uint64) []seat.Seat {
	return func(names []string, seed uint64) []seat.Seat {
		out := defaultSeats(names, seed)
		hs := NewHumanSeat()
		out[0] = hs
		*human = hs
		return out
	}
}

// driveHumanSeat runs the embedder's loop: poll Pending; whenever seat 0 is
// asked a decision, answer it deterministically with the first Min options;
// keep going until the match reaches MatchFinished. It returns once the
// match is finished, and fails the test on a crash or a stall.
//
// Each distinct pending Seq is submitted exactly once: last guards against
// Pending returning a snapshot of a decision that has already been answered
// (the seat's slot is cleared by the parked Decide's defer, which runs after
// the intent is consumed -- so a fast poll can read a decision that was just
// handed to the engine). Submitting the same Seq twice could queue a second
// intent on an already-full channel M2b-2's abandoned-slot design explicitly
// forbids; the Seq guard makes the embedder loop correct for any interleaving.
func driveHumanSeat(t *testing.T, r *Registry, id TableID) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	var last uint64 = ^uint64(0) // a Seq no decision can carry
	for {
		if time.Now().After(deadline) {
			r.Close()
			t.Fatal("human seat did not drive the match to completion within deadline")
		}
		ms, err := r.Matches(id)
		if err != nil {
			r.Close()
			t.Fatalf("Matches: %v", err)
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchCrashed {
				r.Close()
				t.Fatalf("match crashed: %s (%d events)", mi.Head, mi.Events)
			}
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchFinished {
				return
			}
		}
		d, err := r.Pending(id, 1, 0)
		if err != nil {
			// No decision parked for the human seat right now: either the
			// bots are playing their turns or the slot is between re-asks.
			// Keep polling; do not spin hot.
			time.Sleep(time.Millisecond)
			continue
		}
		if d.Seq == last {
			// A re-read of the decision we just answered; wait for the next.
			time.Sleep(time.Millisecond)
			continue
		}
		if err := r.SubmitIntent(id, 1, 0, legalIntent(d)); err != nil {
			r.Close()
			t.Fatalf("SubmitIntent(seq %d): %v", d.Seq, err)
		}
		last = d.Seq
	}
}

// TestHumanSeatPlaysAFullBotTableAndReplaysIdentically is Task M2b-5's one
// acceptance test. It is deliberately not t.Parallel: it is the heaviest
// single match in the suite (four full repo decks), and running it alone in
// the sequential phase keeps it from fighting the parallel tests for a
// match slot, bounding the wall cost it adds to the package budget.
func TestHumanSeatPlaysAFullBotTableAndReplaysIdentically(t *testing.T) {
	// Four of the twelve repo decks, the first in sorted order.
	names := testutil.RepoDeckNames()
	if len(names) < 4 {
		t.Skip("testutil has fewer than 4 repo decks")
	}
	decks := []string{names[0], names[1], names[2], names[3]}

	var human *HumanSeat
	o := testOptions(t)
	o.LoadDeck = repoDeckLoader(t)
	o.Seats = humanFirstSeat(&human)

	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "human-match", Seats: 4, Decks: decks,
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Perpetual: false}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	// (1) The match is driven to completion from the outside.
	driveHumanSeat(t, r, "t1")
	r.Wait("t1")

	ms, err := r.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("matches after driving: %+v (err %v)", ms, err)
	}
	if m := ms[0]; m.Head == "" || m.Events < 100 || m.Turns < 2 {
		t.Fatalf("finished match info implausible: %+v", m)
	}
	t.Logf("human-driven match: %d events, %d turns, head %s", ms[0].Events, ms[0].Turns, ms[0].Head)

	// (2) The caretaker never substituted an answer. With ThinkTimeout 0 and
	// the match completing before Close ever cancels the table context, the
	// only way this counter is non-zero is a decision the human left
	// unanswered -- the exact failure the caretaker exists to paper over
	// (M2b-3, D3). Combined with (1), a finished match and a silent caretaker
	// positively prove the human seat answered every decision it was asked.
	if got := human.caretakerCount(); got != 0 {
		t.Fatalf("the caretaker answered %d decisions; every decision was meant for the human seat", got)
	}

	// (3) The logged game replays byte-identically: a human-driven game is
	// indistinguishable from a recorded one (that is the point of driving the
	// seat through events rather than around them).
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	var fm *match
	if len(tb.history) > 0 {
		fm = tb.history[0]
	}
	tb.mu.RUnlock()
	if fm == nil {
		t.Fatal("the finished match never landed in table history")
	}
	e, err := replay.Replay(fm.e.L, fm.cfg)
	if err != nil {
		t.Fatalf("replay of the human-driven game diverged: %v", err)
	}
	if got, want := e.L.Head(), fm.e.L.Head(); got != want {
		t.Fatalf("replayed chain head %s, recorded %s", got, want)
	}
}
