package host

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/view"
)

// TestSubmitRacingCloseWithUnawaitedParkingReturns pins the play-level cleanup
// for the pace window: the match goroutine installs the next decision's slot,
// publishes it, then calls Sleep before the next iteration's await. A submit
// that claims the slot in that window waits for a verdict. If Close (or the
// maxIntents guard, or a panic outside attempt) ends play before await, the
// parking is never awaited; without the deferred release the submit waits
// forever even though Close returns (D2).
//
// The Sleep hook blocks on stop only once the human seat has a pending
// decision, which is exactly the parked-but-not-yet-awaited window. The test
// then claims the slot with a real SubmitIntent, closes the table, and requires
// the in-flight submit to return.
func TestSubmitRacingCloseWithUnawaitedParkingReturns(t *testing.T) {
	o := testOptions(t)
	o.Seats = twoSeatHumans
	parked := make(chan struct{})
	var r *Registry
	o.Sleep = func(_ time.Duration, stop <-chan struct{}) {
		if _, err := r.Pending("t1", 1, 0); err != nil {
			return
		}
		select {
		case <-parked:
		default:
			close(parked)
		}
		<-stop
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "close-race", Seats: 2, Decks: []string{"a", "b"},
		Seed: 42, Pace: 0, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}

	select {
	case <-parked:
	case <-time.After(20 * time.Second):
		t.Fatal("match never parked a human decision in the pace window")
	}
	d := waitPending(t, r, "t1", 1, 0)

	errc := make(chan error, 1)
	go func() { errc <- r.SubmitIntent("t1", 1, 0, legalIntent(d)) }()

	// The submit claims the slot under the seat mutex before it waits for a
	// verdict, so Pending starts failing exactly when the claim lands. Wait
	// for that claim before closing: it makes the in-flight submit
	// deterministic rather than a race with the abort.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, perr := r.Pending("t1", 1, 0); perr != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("in-flight submit never claimed the parked slot")
		}
		time.Sleep(time.Millisecond)
	}

	r.Close()

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("in-flight submit was accepted after the table closed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("in-flight submit never returned after Close abandoned the unawaited parking")
	}
}
