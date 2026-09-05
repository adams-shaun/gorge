package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
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
	dec := hs.slot.dec
	dec.Options = append([]decision.Option(nil), hs.slot.dec.Options...)
	return &dec, nil
}

// SubmitIntent answers the decision pending on the human seat for player on
// match k of table id. It never drives the engine: it validates the intent
// exactly as decision.Decision.Validate does (stale Seq, wrong player, wrong
// option indices, wrong min-max, duplicates are all rejected — D2) and hands
// it to the parked seat, whose submit wakes the blocked Decide. The actual
// engine Submit still runs inside play() on the match goroutine (D7), so a
// caller who sends on SubmitIntent can never race the log. A rejected intent
// leaves the seat parked and the game exactly where it was.
func (r *Registry) SubmitIntent(id TableID, k int, player state.PlayerID, in decision.Intent) error {
	hs, err := r.humanSeat(id, k, player)
	if err != nil {
		return err
	}
	return hs.submit(in)
}
