package eventhost

import (
	"fmt"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/server/protocol"
)

// advanceBuild drives the BUILD phase: every bot seat saves its builder's
// deck and readies that revision, seats in order. (After the last Ready the
// event is in ROUND1 and advanceRounds takes over.) It reports whether it
// applied anything.
func (r *run) advanceBuild() bool {
	if r.ev.Phase() != event.PhaseBuild {
		return false
	}
	applied := false
	for s := 0; s < draft.Seats; s++ {
		if r.meta.Seats[s].Kind != "bot" {
			continue
		}
		if r.builders[s] == nil {
			return applied
		}
		if r.ev.SavedRev(s) == 0 {
			if _, err := r.saveDeck(s, r.builders[s].Build(r.meta.Set, r.ev.Pool(s))); err != nil {
				// A bot's own deck failing to validate is a builder
				// defect: halt rather than stall in BUILD forever.
				if r.halted == "" {
					r.halted = fmt.Sprintf("event %s: bot seat %d could not save its deck: %v", r.meta.ID, s, err)
				}
				return applied
			}
			applied = true
		}
		if r.ev.ReadyRev(s) == 0 {
			if err := r.apply(event.Entry{Kind: event.KindReady, Seat: s, Rev: r.ev.SavedRev(s)}); err != nil {
				return applied
			}
			applied = true
		}
	}
	return applied
}

// deckSaveOpen reports whether seat may save a deck revision now: BUILD,
// or — E3-5 — a round phase whose seat's series is between games with the
// seat's side still to submit its deck (the sideboard UI saves a new
// revision in the between window).
func (r *run) deckSaveOpen(seat int) bool {
	switch r.ev.Phase() {
	case event.PhaseBuild:
		return true
	case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
	default:
		return false
	}
	a, b, ok := r.ev.SeriesOf(seat)
	if !ok {
		return false
	}
	v, ok := r.ev.SeriesState(a, b)
	if !ok || !v.Between {
		return false
	}
	side := 0
	if seat == b {
		side = 1
	}
	return v.Locked[side] == 0
}

// saveDeck validates one seat's deck against its pool and persists the
// revision: the deck file FIRST, then the DeckSaved entry — a crash between
// the two leaves an orphan file that the next save of that revision
// overwrites. The event must be in BUILD (or the seat must be in its
// series' between window, E3-5); an illegal deck is invalid_deck.
func (r *run) saveDeck(seat int, d build.Deck) (int, error) {
	if !r.deckSaveOpen(seat) {
		return 0, errConflict("eventhost: the event is not building (phase %s)", r.ev.Phase())
	}
	if err := build.Validate(r.ev.Pool(seat), d); err != nil {
		return 0, errInvalidDeck("eventhost: %v", err)
	}
	rev := r.ev.SavedRev(seat) + 1
	name := fmt.Sprintf("%s %s draft (seat %d)", r.meta.ID, r.meta.Set, seat+1)
	if err := r.h.opts.Store.writeDeck(r.meta.ID, seat, rev, build.ToDeckFile(r.ev.Pool(seat), d, name)); err != nil {
		r.halted = fmt.Sprintf("event %s: persistence failed: %v", r.meta.ID, err)
		return 0, errUnavailable("%s", r.halted)
	}
	if err := r.apply(event.Entry{Kind: event.KindDeckSaved, Seat: seat, Rev: rev, Deck: &d}); err != nil {
		return 0, err
	}
	return rev, nil
}

// EventPool is the seat's drafted pool, its latest saved deck revision (nil
// before the first save), whether it has readied, and the bot builder's
// suggested deck for the pool — the suggestion only once the draft is over
// (BUILD or later; zero value during LOBBY and DRAFT).
func (h *Host) EventPool(id, token string) (protocol.EventPool, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.EventPool{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.EventPool{}, terr
	}
	if r.ev.Phase() == event.PhaseLobby {
		return protocol.EventPool{}, errConflict("eventhost: the event has not drafted yet (phase %s)", r.ev.Phase())
	}
	return r.poolFor(seat), nil
}

// poolFor builds one seat's EventPool.
func (r *run) poolFor(seat int) protocol.EventPool {
	out := protocol.EventPool{
		Event: r.meta.ID, Seat: seat,
		Pool:  r.draftCards(r.ev.Pool(seat)),
		Rev:   r.ev.SavedRev(seat),
		Ready: r.ev.ReadyRev(seat) != 0,
	}
	if d, ok := r.ev.SavedDeck(seat); ok {
		out.Deck = &protocol.DeckRequest{Main: d.Main, Basics: d.Basics}
	}
	// E3-5: in a round, Playing is the deck the seat plays (or last
	// played) in its series' current games, with PlayingRev naming the
	// revision — the sideboard UI's starting point.
	switch r.ev.Phase() {
	case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
		if a, b, ok := r.ev.SeriesOf(seat); ok {
			side := 0
			if seat == b {
				side = 1
			}
			if v, ok := r.ev.SeriesState(a, b); ok && v.PlayRevs[side] != 0 {
				if d, ok := r.ev.DeckRev(seat, v.PlayRevs[side]); ok {
					out.Playing = &protocol.DeckRequest{Main: d.Main, Basics: d.Basics}
					out.PlayingRev = v.PlayRevs[side]
				}
			}
		}
	}
	ph := r.ev.Phase()
	if r.builders[seat] != nil && ph != event.PhaseLobby && ph != event.PhaseDraft {
		s := r.builders[seat].Build(r.meta.Set, r.ev.Pool(seat))
		out.Suggestion = protocol.DeckRequest{Main: s.Main, Basics: s.Basics}
	}
	return out
}

// CheckDeck validates a deck request against the seat's pool and answers
// the auto-land split the builder would add. It never logs anything and is
// allowed from DRAFT on (the pool exists from the first PackOpened).
func (h *Host) CheckDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.DeckCheck{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.DeckCheck{}, terr
	}
	if r.ev.Phase() == event.PhaseLobby {
		return protocol.DeckCheck{}, errConflict("eventhost: the event has not drafted yet (phase %s)", r.ev.Phase())
	}
	// No markAct here: CheckDeck is a read-only check, and a client that
	// only polls it must not keep an otherwise-idle event live past its
	// idle window. Only state-changing actions (join, pick, save, ready)
	// refresh the window.
	return r.deckCheck(seat, req), nil
}

// deckCheck is the DeckCheck body for one seat's request.
func (r *run) deckCheck(seat int, req protocol.DeckRequest) protocol.DeckCheck {
	d := build.Deck{Main: req.Main, Basics: req.Basics}
	out := protocol.DeckCheck{Total: d.Total(), AutoLands: build.AutoLands(req.Main, r.pipsOf(seat))}
	if err := build.Validate(r.ev.Pool(seat), d); err != nil {
		out.Error = err.Error()
		return out
	}
	out.Valid = true
	return out
}

// SaveDeck saves one seat's deck revision, then lets the bots answer and
// reports the same deck check CheckDeck would, with the new revision.
func (h *Host) SaveDeck(id, token string, req protocol.DeckRequest) (protocol.DeckCheck, error) {
	r, ferr := h.find(id)
	if ferr != nil {
		return protocol.DeckCheck{}, ferr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	seat, terr := r.seatFor(token, false)
	if terr != nil {
		return protocol.DeckCheck{}, terr
	}
	if r.halted != "" {
		return protocol.DeckCheck{}, errUnavailable("%s", r.halted)
	}
	rev, serr := r.saveDeck(seat, build.Deck{Main: req.Main, Basics: req.Basics})
	if serr != nil {
		return protocol.DeckCheck{}, serr
	}
	r.advance()
	r.markAct(seat, r.h.clockNow())
	r.broadcast()
	out := r.deckCheck(seat, req)
	out.Rev = rev
	return out, nil
}

// Ready readies the seat's saved revision and, when it was the last seat,
// lets the event open round 1. Re-readying the readied revision is an
// idempotent success.
func (h *Host) Ready(id, token string, req protocol.ReadyRequest) (protocol.EventState, error) {
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
	if r.ev.Phase() != event.PhaseBuild {
		return protocol.EventState{}, errConflict("eventhost: the event is not building (phase %s)", r.ev.Phase())
	}
	if req.Rev <= 0 || req.Rev != r.ev.SavedRev(seat) {
		return protocol.EventState{}, errConflict("eventhost: seat %d's latest saved revision is %d, not %d",
			seat, r.ev.SavedRev(seat), req.Rev)
	}
	if r.ev.ReadyRev(seat) != req.Rev {
		if err := r.apply(event.Entry{Kind: event.KindReady, Seat: seat, Rev: req.Rev}); err != nil {
			return protocol.EventState{}, err
		}
		r.advance()
		r.broadcast()
	}
	r.markAct(seat, r.h.clockNow())
	return r.stateFor(seat), nil
}
