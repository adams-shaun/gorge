package eventhost

import (
	"github.com/adams-shaun/gorge/draft/event"
)

// keepRev is the revision a side keeps when it does not rebuild: in the Bo3
// between-games window the deck it played last game (PlayRevs), in the
// rebuild window the deck it played last (LastPlayedRev).
func (r *run) keepRev(v event.SeriesView, seat, side int) int {
	if v.Between {
		return v.PlayRevs[side]
	}
	return r.ev.LastPlayedRev(seat)
}

// lockOpening advances one series' rebuild window by at most one entry: the
// first unlocked bot side (in series order) locks keepRev with a Sideboarded
// (Game 1) entry, one per advance pass exactly as the Bo3 between window
// locks its bots. An unlocked human side does not block a later bot side:
// the window is open for the human alone once the bots have locked. It
// reports applied (an entry was logged) and waiting (the window is still
// open after this call: a human side has not locked yet). Outside the window
// it reports false, false, nil. The caller holds r.mu.
func (r *run) lockOpening(sp event.SeriesSpec) (applied, waiting bool, err error) {
	v, ok := r.ev.SeriesState(sp.A, sp.B)
	if !ok || !v.Opening {
		return false, false, nil
	}
	for side, seat := range []int{sp.A, sp.B} {
		if v.Locked[side] != 0 {
			continue
		}
		if r.meta.Seats[seat].Kind != "bot" {
			waiting = true // a human side waits until it sideboards
			continue
		}
		if err := r.apply(event.Entry{
			Kind: event.KindSideboarded, Round: sp.Round, A: sp.A, B: sp.B,
			Game: 1, Seat: seat, Rev: r.keepRev(v, seat, side),
		}); err != nil {
			return false, waiting, err
		}
		return true, waiting, nil
	}
	return false, waiting, nil // both sides locked: the window is ready to close
}
