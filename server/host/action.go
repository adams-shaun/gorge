package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/protocol"
)

// humanSeat resolves (id, k, player) to the live match's HumanSeat at that
// seat. It returns a clear error for a missing table or match, a non-live
// (finished/crashed/archived) match, an out-of-range player, and a seat
// that is a bot rather than a *HumanSeat — never a panic and never a silent
// no-op. It reads the match lock only long enough to copy the seats slice
// and stash the live flag, then releases it: play() installs m.slots once
// and never reassigns it, so the returned *HumanSeat is stable past the
// lock and can be used without holding m.mu (readers must never drive the
// engine, D7, and a seat's channel send must not happen under a match lock).
func (r *Registry) humanSeat(id TableID, k int, player state.PlayerID) (*HumanSeat, error) {
	_, m, err := r.lookup(id, k)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	st := m.state
	slots := m.slots
	m.mu.RUnlock()
	if st != protocol.MatchLive {
		return nil, fmt.Errorf("host: match %d is %s, nothing pending", k, st)
	}
	if slots == nil {
		// newMatch publishes the match live a moment before play() installs
		// its seats; a poll in that window finds nothing parked yet.
		return nil, fmt.Errorf("host: match %d: no decision pending for player %d (seats not installed yet)", k, player)
	}
	if int(player) >= len(slots) {
		return nil, fmt.Errorf("host: match %d: player %d out of range (%d seats)", k, player, len(slots))
	}
	hs, ok := slots[player].(*HumanSeat)
	if !ok {
		return nil, fmt.Errorf("host: match %d: seat %d is not a human seat", k, player)
	}
	return hs, nil
}

// Pending returns a copy of the decision currently being asked of the human
// seat for player on match k of table id, or an error when the match/seat
// cannot answer (missing table, non-live match, out-of-range player, a bot
// seat, or no decision parked). The Options slice is copied so a caller can
// never mutate the seat's own pending decision through it.
func (r *Registry) Pending(id TableID, k int, player state.PlayerID) (*decision.Decision, error) {
	hs, err := r.humanSeat(id, k, player)
	if err != nil {
		return nil, err
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.slot == nil {
		return nil, fmt.Errorf("host: no decision pending for player %d", player)
	}
	return hs.slot.dec.Clone(), nil
}

// SubmitIntent answers the decision pending on the human seat for player on
// match k of table id. It never drives the engine: it validates the intent
// exactly as decision.Decision.Validate does (stale Seq, wrong player, wrong
// option indices, wrong min-max, duplicates are all rejected — D2) and hands
// it to the parked seat. The call blocks until the match goroutine has
// attempted the engine Submit: it returns nil once the engine accepted the
// intent, or the engine's refusal as its error (the HTTP layer's 409), with
// the decision left pending for a legal answer. The actual engine Submit runs
// inside play() on the match goroutine (D7), so a caller who sends on
// SubmitIntent can never race the log.
func (r *Registry) SubmitIntent(id TableID, k int, player state.PlayerID, in decision.Intent) error {
	return r.SubmitIntentFrom(id, k, player, in, false)
}

// SubmitIntentFrom is SubmitIntent with HD-2's client-autopilot bit: auto
// marks the answer as produced by the client's autopilot rather than a
// deliberate human choice (docs/design/2026-10-10-human-divergence-design.md
// §5.2). The flag travels to the parked slot — never into decision.Intent,
// whose values are replay input — and the play loop records it in the match's
// provenance ledger (server/host/divergence_ledger.go), so the divergence capture can
// exclude it. SubmitIntent is the auto=false form.
func (r *Registry) SubmitIntentFrom(id TableID, k int, player state.PlayerID, in decision.Intent, auto bool) error {
	hs, err := r.humanSeat(id, k, player)
	if err != nil {
		return err
	}
	if in.Payment == nil && in.Announce == nil {
		return hs.submitAdmitted(in, nil, auto)
	}
	_, m, err := r.lookup(id, k)
	if err != nil {
		return err
	}
	if in.Announce != nil {
		// Announce then pay (docs/design/2026-09-27-announce-then-pay.md
		// §3.1): Decision.Validate proves the selector names an offered action
		// that carries a plan; the engine's independent re-plan proves the cast
		// is still payable before the parked match is woken.
		return hs.submitAdmitted(in, func(d decision.Decision, admitted decision.Intent) error {
			var action *decision.PaymentAction
			for i := range d.PaymentActions {
				if d.PaymentActions[i].ID == admitted.Announce.ActionID {
					action = &d.PaymentActions[i]
					break
				}
			}
			if action == nil {
				return fmt.Errorf("host: payment action is not offered")
			}
			return m.locked(func() error {
				if m.state != protocol.MatchLive {
					return fmt.Errorf("host: match %d is %s, nothing pending", k, m.state)
				}
				if pp, ok := m.g.(game.PaymentPlanner); ok {
					if err := pp.ValidateCastAnnounce(player, action.Cast); err != nil {
						return fmt.Errorf("host: announce rejected: %w", err)
					}
				}
				return nil
			})
		}, auto)
	}
	return hs.submitAdmitted(in, func(d decision.Decision, admitted decision.Intent) error {
		// Decision.Validate above proves this selector names an offered action
		// and verbatim offered witness.  It cannot prove the witness against
		// the live rules state: it intentionally has no engine dependency.
		// Do that proof here, before the channel send is allowed to acknowledge
		// the intent to a parked match. A game that is no PaymentPlanner has no
		// such proof, so Validate stands alone.
		var action *decision.PaymentAction
		for i := range d.PaymentActions {
			if d.PaymentActions[i].ID == admitted.Payment.ActionID {
				action = &d.PaymentActions[i]
				break
			}
		}
		if action == nil {
			return fmt.Errorf("host: payment action is not offered")
		}
		return m.locked(func() error {
			if m.state != protocol.MatchLive {
				return fmt.Errorf("host: match %d is %s, nothing pending", k, m.state)
			}
			if pp, ok := m.g.(game.PaymentPlanner); ok {
				if err := pp.ValidateCastPayment(player, action.Cast, admitted.Payment.Plan); err != nil {
					return fmt.Errorf("host: payment plan rejected: %w", err)
				}
			}
			return nil
		})
	}, auto)
}
