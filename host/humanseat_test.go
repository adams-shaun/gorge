package host

import (
	"context"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/view"
)

func TestHumanSeatBlocksUntilSubmittable(t *testing.T) {
	s := NewHumanSeat()
	d := decision.Decision{Seq: 3, Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Label: "pass"}, {Index: 1, Label: "act"}}}
	done := make(chan decision.Intent, 1)
	errc := make(chan error, 1)
	go func() {
		in, err := s.Decide(context.Background(), view.View{}, d)
		if err == nil {
			done <- in
		} else {
			errc <- err
		}
	}()
	// No answer yet: Decide must be blocked.
	select {
	case <-done:
		t.Fatal("Decide returned before an intent was submitted")
	case <-time.After(20 * time.Millisecond):
	}
	if ok, got := s.pending(); !ok || got.Seq != 3 {
		t.Fatalf("pending() = %v, %+v", ok, got)
	}
	if err := s.submit(decision.Intent{Seq: 3, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	select {
	case in := <-done:
		if len(in.Choices) != 1 || in.Choices[0] != 0 {
			t.Fatalf("intent %+v", in)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Decide did not return after submit")
	}
}

func TestHumanSeatSubmitRejectsWrongSeqAndUnblocksOnCancel(t *testing.T) {
	s := NewHumanSeat()
	d := decision.Decision{Seq: 9, Player: 0, Min: 1, Max: 1, Options: []decision.Option{{Index: 0}}}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { _, err := s.Decide(ctx, view.View{}, d); errc <- err }()
	if err := s.submit(decision.Intent{Seq: 8, Player: 0, Choices: []int{0}}); err == nil {
		t.Error("submit with a stale Seq was accepted")
	}
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("cancel did not error Decide")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("cancel did not unblock Decide")
	}
}

// TestACancelledDecisionDoesNotAnswerTheNextOne drives the abandoned-slot
// sequence: a decision is cancelled, a late submit arrives for it, and then a
// NEW decision is asked. The new decision must still block -- the stale intent
// belongs to a decision nobody is waiting on any more.
func TestACancelledDecisionDoesNotAnswerTheNextOne(t *testing.T) {
	s := NewHumanSeat()
	d1 := decision.Decision{Seq: 11, Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Label: "pass"}, {Index: 1, Label: "act"}}}
	ctx1, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { _, err := s.Decide(ctx1, view.View{}, d1); errc <- err }()
	// Wait until Decide is genuinely parked before cancelling.
	deadline := time.Now().Add(1 * time.Second)
	for {
		if ok, _ := s.pending(); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Decide never parked")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("cancel did not error Decide")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("cancel did not unblock Decide")
	}

	// (a) After the cancelled Decide returned, no decision is pending.
	if ok, got := s.pending(); ok {
		t.Fatalf("pending() = (%v, %+v) after the decision was cancelled; want no pending decision", ok, got)
	}

	// (b) A late submit carrying the abandoned decision's Seq must be rejected.
	if err := s.submit(decision.Intent{Seq: d1.Seq, Player: 1, Choices: []int{1}}); err == nil {
		t.Fatal("submit for the cancelled decision was accepted")
	}

	// (c) A fresh Decide with d2 must block: the stale intent must not answer it.
	d2 := decision.Decision{Seq: 12, Player: 1, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Label: "pass"}, {Index: 1, Label: "act"}}}
	done := make(chan decision.Intent, 1)
	go func() { in, _ := s.Decide(context.Background(), view.View{}, d2); done <- in }()
	select {
	case in := <-done:
		t.Fatalf("Decide answered %+v without a submit -- a stale intent leaked to the next decision", in)
	case <-time.After(30 * time.Millisecond):
		// blocked: as expected.
	}
	// Answer d2 properly; the returned intent must be d2's, not d1's.
	if err := s.submit(decision.Intent{Seq: d2.Seq, Player: 1, Choices: []int{1}}); err != nil {
		t.Fatalf("submit d2: %v", err)
	}
	select {
	case in := <-done:
		if in.Seq != d2.Seq || len(in.Choices) != 1 || in.Choices[0] != 1 {
			t.Fatalf("got %+v, want d2's intent", in)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Decide did not return after submitting d2's intent")
	}
}
