package host

// Defect B regression: a pending decision used to be public — on the /view
// snapshot and the SSE stream — for a whole `pace` before the seat that owns
// it could accept an answer, so an intent posted in that window was rejected
// with "no pending decision". The run loop ended every iteration publish →
// sleep(pace) → park-the-seat, so for Pace=250ms a client saw a decision for
// ~250ms before it could be answered (measured: /view→/pending disagreement
// of 242–253ms every window; posting off /view gave accepted=1, refused=19).
//
// The fix parks the decision (installs the seat's accept-ready slot) before
// it is published, so no window remains. This test drives the REAL loop with
// an injected Options.Sleep — the seam play() uses for Pace — and, from
// INSIDE the per-decision pace sleep (the exact published-but-unanswered
// window the defect was), reads the current pending decision exactly as a
// client would and answers it. On main the seat is not yet parked when its
// decision is on the wire, so the answer is rejected and the match wedges
// after that decision — the test observes the rejection, counts it, and
// fails. The poll loop, not r.Wait, closes the test, because r.Wait cannot
// complete under the defect (the match never finishes once answers stop
// being accepted). A timing test proven by a time.Sleep is not proof; this
// one is synchronous inside the seam, so it is deterministic.
//
// fails/seenHuman are guarded because the match goroutine writes them (inside
// the injected Sleep) while the test goroutine polls them.

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

func TestPublishedDecisionIsAnswerableTheMomentItPublishes(t *testing.T) {
	t.Parallel()
	const pace = 50 * time.Millisecond

	var (
		mu        sync.Mutex
		fails     int
		seenHuman int
	)
	var r *Registry
	// primed gates the hook off until the test has submitted the prime below.
	// The prime exists because the pre-toss engine's FIRST decision belonged
	// to seat 0 and had no pace sleep before it, so the hook could not have
	// answered it -- the test had to. Under the CR 103.1 toss the toss winner
	// may be the bot (it is, at seed 42's match seed): seat 0's first decision
	// now follows a slept decision, and without the gate the hook and the
	// prime would both answer it, one of them rejected. The gate is checked
	// at hook entry, so the hook either skips the primed decision entirely
	// (primed still false) or runs only on later decisions (primed true means
	// the prime's SubmitIntent already happened).
	var primed atomic.Bool // written by the test goroutine, read on the match goroutine
	o := testOptions(t)
	o.Seats = twoSeatHumans // seat 0 human, seat 1 bot
	o.Sleep = func(d time.Duration, stop <-chan struct{}) {
		if d != pace || !primed.Load() {
			return // only the per-decision pace sleep is the defect-B window
		}
		// We are on the match goroutine, inside the pace sleep that follows
		// park + fan-out, holding no match lock. Read what a client sees: the
		// pending decision the fan-out just published.
		r.mu.RLock()
		tb := r.tables["t1"]
		r.mu.RUnlock()
		if tb == nil {
			return
		}
		tb.mu.RLock()
		m := tb.cur
		tb.mu.RUnlock()
		if m == nil {
			return
		}
		m.mu.RLock()
		pd := m.e.Pending()
		m.mu.RUnlock()
		if pd == nil || pd.Player != 0 {
			return // game ending, or the bot owns this decision
		}
		// The human's decision is on the wire right now. The seat must already
		// be accept-ready: publishing must never precede parking. Any error
		// here is the exact defect — a decision visible but unanswerable.
		answer, perr := r.Pending("t1", 1, 0)
		if perr != nil {
			mu.Lock()
			fails++
			mu.Unlock()
			return
		}
		if serr := r.SubmitIntent("t1", 1, 0, legalIntent(answer)); serr != nil {
			mu.Lock()
			fails++
			mu.Unlock()
			return
		}
		mu.Lock()
		seenHuman++
		mu.Unlock()
	}

	r, _ = New(o)
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "race", Seats: 2, Decks: []string{"a", "b"},
		Seed: 42, Pace: pace, Spectator: view.Omniscient, Perpetual: false}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	// Drive the FIRST human decision from outside: the hook above is gated
	// off until this SubmitIntent completes, so it cannot answer the decision
	// the prime is answering; every human decision from there is answered
	// inside the injected Sleep.
	d0 := waitPending(t, r, "t1", 1, 0)
	if err := r.SubmitIntent("t1", 1, 0, legalIntent(d0)); err != nil {
		t.Fatalf("prime SubmitIntent: %v", err)
	}
	primed.Store(true)

	// Poll for the outcome rather than r.Wait, which cannot complete under the
	// defect: a rejected human answer wedges the match. Detect the bug as soon
	// as a submission is refused; otherwise wait for the match to finish, then
	// assert it finished cleanly and was not vacuous.
	deadline := time.Now().Add(90 * time.Second)
	var ms []protocol.MatchInfo
	for {
		mu.Lock()
		if fails > 0 {
			f := fails
			mu.Unlock()
			t.Fatalf("defect B: %d window(s) where a published decision was visible but unanswerable", f)
		}
		h := seenHuman
		mu.Unlock()
		ms, _ = r.Matches("t1")
		live := false
		for _, mi := range ms {
			if mi.State == protocol.MatchLive {
				live = true
			}
		}
		if !live {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("match did not finish within deadline (fails=%d seenHuman=%d)", 0, h)
		}
		time.Sleep(2 * time.Millisecond)
	}
	mu.Lock()
	h := seenHuman
	mu.Unlock()
	if h == 0 {
		t.Fatal("test observed no human-published decision; not a valid regression run")
	}
	for _, mi := range ms {
		if mi.State == protocol.MatchCrashed {
			t.Fatalf("match crashed while closing the publish/park window: %s (%d events)", mi.Head, mi.Events)
		}
	}
}
