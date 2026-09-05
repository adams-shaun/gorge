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
