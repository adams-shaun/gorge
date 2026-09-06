package host

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// HumanSeat is a seat.Seat that can hold one decision pending and be answered
// from outside (Task M2b-2's SubmitIntent). It is "just another seat.Seat":
// the engine hands it the same view.View and decision.Decision it hands any
// bot, and blocks on Decide until an intent is submitted, ctx is cancelled, or
// ThinkTimeout elapses. It carries a caretaker seat (Task M2b-3): the
// deterministic bot that would have occupied this slot in a pure-bot game, so
// an unanswered decision is turned by exactly the intent that bot would have
// produced and the logged game replays byte-identically (D3).
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
	// timeout is the per-decision think budget in force (Options.ThinkTimeout
	// at the time play configured this seat). 0 means no timeout: the seat
	// waits for a submit or ctx cancel as it always did.
	timeout time.Duration
	// caretaker is the deterministic bot that answers a timed-out or
	// ctx-cancelled decision in the player's place. Configuring walk sets it
	// to the bot defaultSeats would have built for this slot.
	caretaker seat.Seat
	// caretakerFires counts how many decisions this seat has delegated to its
	// caretaker. Task M2b-5 reads it to assert positively that a human who
	// answered every decision was never substituted for. Without a counter it
	// would be unobservable: by design D3 a caretaker's intent is byte-identical
	// to the human's, so absence-of-something is the only signal available, and
	// a counter is that signal.
	caretakerFires uint64
}

// configure arms the seat with its caretaker and think budget. play calls it
// once per match, on the match goroutine, before any Decide, so it never
// races a parked Decide; the fields are copied under the mutex in Decide
// (M2b-2's copy-under-the-lock shape). A zero timeout still installs the
// caretaker so ctx cancellation falls back to it.
func (s *HumanSeat) configure(timeout time.Duration, caretaker seat.Seat) {
	s.mu.Lock()
	s.timeout = timeout
	s.caretaker = caretaker
	s.mu.Unlock()
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
// is submitted, ctx is done, or ThinkTimeout elapses. It satisfies seat.Seat.
// Task M2b-3 wires the two exit-but-still-playable paths (timeout and ctx
// cancel) to the caretaker: instead of erroring (which would crash the match
// and, worse, leave a disconnected human able to wedge play's blocked Decide
// forever -- FL-17), the seat answers with the deterministic bot's intent for
// that slot, which play submits like any other, so the logged game continues as
// if the slot had been a bot all along (D3). A human reconnect still works via
// SubmitIntent against the oncoming decisions (D2). With no caretaker (a bare
// NewHumanSeat, as today's unit tests build) or a zero ThinkTimeout, ctx
// cancellation alone is what unblocks Decide and returns ctx.Err().
//
// Every return path -- human answer, caretaker answer and cancellation alike
// -- clears the seat's slot, but only if it is still this call's own slot:
// identity comparison stops an earlier Decide from un-publishing the slot an
// overlapping later Decide installed (Decide is not called concurrently
// today, but defensive here costs nothing). With the slot gone, a later submit
// finds no current decision to answer and is rejected, and the next Decide
// parks on a brand-new channel. The caretaker path is no exception: it clears
// the slot through the same defer, so after a timeout the seat is clean for
// the next decision -- no leftover slot, no buffered intent.
func (s *HumanSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	slot := &pendingSlot{dec: d, recv: make(chan decision.Intent, 1)}
	s.mu.Lock()
	s.slot = slot
	timeout := s.timeout
	caretaker := s.caretaker
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.slot == slot {
			s.slot = nil
		}
		s.mu.Unlock()
	}()

	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case in := <-slot.recv:
			return in, nil
		case <-ctx.Done():
			if caretaker != nil {
				return s.viaCaretaker(ctx, v, d)
			}
			return decision.Intent{}, ctx.Err()
		case <-timer.C:
			if caretaker != nil {
				return s.viaCaretaker(ctx, v, d)
			}
			// A timeout configured but no caretaker to fall back to is an
			// unarmed seat (unreachable when play configured it); rather than
			// error a decision nobody asked us to abandon, drop through to the
			// blocking select and keep waiting on submit/ctx.
		}
	}
	select {
	case in := <-slot.recv:
		return in, nil
	case <-ctx.Done():
		if caretaker != nil {
			return s.viaCaretaker(ctx, v, d)
		}
		return decision.Intent{}, ctx.Err()
	}
}

// viaCaretaker records a caretaker substitution and performs it. The counter
// is Task M2b-5's positive signal that a human answered every decision: it is
// incremented under the same mutex that guards the seat's other fields, before
// the (deterministic) caretaker intent is produced, so a test can read it only
// after Decide has returned (the match goroutine is the sole writer).
func (s *HumanSeat) viaCaretaker(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.mu.Lock()
	s.caretakerFires++
	s.mu.Unlock()
	return s.caretaker.Decide(ctx, v, d)
}

// caretakerCount reports how many decisions the caretaker has answered in
// this seat's place. 0 after a full match means every decision was answered
// by a human submit. Package-internal; Task M2b-5 reads it to assert the
// caretaker never fired.
func (s *HumanSeat) caretakerCount() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caretakerFires
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
