package eventhost

import (
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/server/protocol"
)

// Sideboard submits one human seat's deck for its current series' next
// game. Keep re-locks the revision the seat played last game; a
// Deck request is validated against the seat's pool and saved as a new
// revision first. Exactly one of the two is set. The series must be in its
// between window, the request's Game must be the series' next game, and
// the seat must not have sideboarded yet; every other state is a 409.
func (h *Host) Sideboard(id, token string, req protocol.SideboardRequest) (protocol.EventState, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.EventState{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.EventState{}, terr
	}
	if r.halted != "" {
		return protocol.EventState{}, errUnavailable("%s", r.halted)
	}
	switch r.ev.Phase() {
	case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
	default:
		return protocol.EventState{}, errConflict("eventhost: the event is not in a round (phase %s)", r.ev.Phase())
	}
	if req.Keep == (req.Deck != nil) {
		return protocol.EventState{}, errBadRequest("eventhost: set exactly one of keep and deck")
	}
	a, b, ok := r.ev.SeriesOf(seat)
	if !ok {
		return protocol.EventState{}, errConflict("eventhost: seat %d is not in a series this round", seat)
	}
	v, ok := r.ev.SeriesState(a, b)
	if !ok || !v.Between {
		return protocol.EventState{}, errConflict("eventhost: seat %d's series is not between games", seat)
	}
	if req.Game != v.NextGame {
		return protocol.EventState{}, errConflict("eventhost: seat %d's series next game is %d, not %d",
			seat, v.NextGame, req.Game)
	}
	side := 0
	if seat == b {
		side = 1
	}
	if v.Locked[side] != 0 {
		return protocol.EventState{}, errConflict("eventhost: seat %d already sideboarded for game %d", seat, req.Game)
	}
	rev := v.PlayRevs[side]
	if !req.Keep {
		nrev, serr := r.saveDeck(seat, build.Deck{Main: req.Deck.Main, Basics: req.Deck.Basics})
		if serr != nil {
			return protocol.EventState{}, serr
		}
		rev = nrev
	}
	if err := r.apply(event.Entry{
		Kind: event.KindSideboarded, Round: v.Round, A: a, B: b,
		Game: v.NextGame, Seat: seat, Rev: rev,
	}); err != nil {
		return protocol.EventState{}, err
	}
	r.advance()
	r.markAct(seat, r.h.clockNow())
	r.broadcast()
	return r.stateFor(seat), nil
}
