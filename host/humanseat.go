package host

import (
	"context"
	"fmt"
	"sync"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/view"
)

// HumanSeat is a seat.Seat that can hold one decision pending and be answered
// from outside (Task M2b-2's SubmitIntent). It is "just another seat.Seat":
// the engine hands it the same view.View and decision.Decision it hands any
// bot, and blocks on Decide until an intent is submitted or ctx is cancelled.
type HumanSeat struct {
	mu sync.Mutex
	// p is the decision currently being answered; nil when Decide is not
	// parked (or has already been answered). It lets pending() inspect the
	// live decision and submit validate against the right Seq.
	p *decision.Decision
	// recv is the slot a matching submit wakes Decider on. Buffered so a
	// submit racing the ctx cancellation is never lost.
	recv chan decision.Intent
}

// NewHumanSeat returns a HumanSeat with no decision pending.
func NewHumanSeat() *HumanSeat {
	return &HumanSeat{recv: make(chan decision.Intent, 1)}
}

// Decide records d as the pending decision and blocks until a matching intent
// is submitted or ctx is done. It satisfies seat.Seat. Task M2b-3 wires ctx to
// the caretaker/think-timeout; today a cancellation simply errors Decide, which
// is exactly what lets a caller break a parked match goroutine (the failure
// this design is judged on -- a seat that can park forever with no way out).
func (s *HumanSeat) Decide(ctx context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	c := decision.Intent{}
	s.mu.Lock()
	s.p = &d
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return c, ctx.Err()
	case in := <-s.recv:
		return in, nil
	}
}

// pending returns whether a decision is currently being answered, and the
// decision itself. Package-internal; Task M2b-2's Pending/SubmitIntent use it.
// (Signature is (bool, *decision.Decision) -- ok first -- to match the task's
// step-1 test `if ok, got := s.pending(); !ok || got.Seq != 3`.)
func (s *HumanSeat) pending() (bool, *decision.Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return false, nil
	}
	return true, s.p
}

// submit delivers an intent to a parked Decide, refusing anything that does not
// match the pending decision. It runs decision.Decision.Validate so only valid
// intents wake the seat: wrong Seq, wrong player, wrong option indices,
// wrong min-max, or duplicates are all rejected without unblocking Decide
// (the engine is free to have moved on; a stale answer must not clobber it).
func (s *HumanSeat) submit(in decision.Intent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return fmt.Errorf("no pending decision")
	}
	if err := s.p.Validate(in); err != nil {
		return err
	}
	s.p = nil
	s.recv <- in
	return nil
}
