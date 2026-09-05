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
	// slot is the decision currently being answered, together with the recv
	// channel that submits to it; nil when Decide is not parked (or has
	// already returned). Task M2b-1's design decision D1: each Decide call
	// gets its own slot, so a recv channel belongs to exactly one decision
	// and cannot outlive her -- a late submit for an abandoned decision goes
	// to a channel nobody will ever read again, instead of poisoning the next
	// decision (the abandoned-slot leak this fix round is named for).
	slot *pendingSlot
}

// pendingSlot binds one decision to the recv channel that answers it. Keep
// dec by value so a mechanical copy has its own decision instance, and keep
// recv per-call (capacity 1) so a racing submit is never silently dropped.
type pendingSlot struct {
	dec  decision.Decision
	recv chan decision.Intent
}

// NewHumanSeat returns a HumanSeat with no decision pending.
func NewHumanSeat() *HumanSeat {
	return &HumanSeat{}
}

// Decide records d as the pending decision and blocks until a matching intent
// is submitted or ctx is done. It satisfies seat.Seat. Task M2b-3 wires ctx to
// the caretaker/think-timeout; today a cancellation simply errors Decide, which
// is exactly what lets a caller break a parked match goroutine (the failure
// this design is judged on -- a seat that can park forever with no way out).
//
// Every return path -- success and cancellation alike -- clears the seat's
// slot, but only if it is still this call's own slot: identity comparison
// stops an earlier Decide from un-publishing the slot an overlapping later
// Decide installed (Decide is not called concurrently today, but defensive
// here costs nothing). With the slot gone, a later submit finds no current
// decision to answer and is rejected, and the next Decide parks on a brand-new
// channel.
func (s *HumanSeat) Decide(ctx context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	slot := &pendingSlot{dec: d, recv: make(chan decision.Intent, 1)}
	s.mu.Lock()
	s.slot = slot
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.slot == slot {
			s.slot = nil
		}
		s.mu.Unlock()
	}()
	c := decision.Intent{}
	select {
	case <-ctx.Done():
		return c, ctx.Err()
	case in := <-slot.recv:
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
	if s.slot == nil {
		return false, nil
	}
	return true, &s.slot.dec
}

// submit delivers an intent to a parked Decide, refusing anything that does not
// match the pending decision. It runs decision.Decision.Validate so only valid
// intents wake the seat: wrong Seq, wrong player, wrong option indices,
// wrong min-max, or duplicates are all rejected without unblocking Decide
// (the engine is free to have moved on; a stale answer must not clobber it).
// The slot pointer is copied under the mutex but the send happens OUTSIDE it,
// so a send can never block a holder of s.mu (a leaked per-decision send would
// otherwise deadlock pending() and the next Decide).
func (s *HumanSeat) submit(in decision.Intent) error {
	s.mu.Lock()
	if s.slot == nil {
		s.mu.Unlock()
		return fmt.Errorf("no pending decision")
	}
	slot := s.slot
	if err := slot.dec.Validate(in); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	slot.recv <- in
	return nil
}
