package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// mulliganRound is the London mulligan round's plain-value state, built by
// New between the opening deal and turn 1 and driven one decision at a time
// by stepPregame (rules/turn.go's step dispatches here while e.pregame). It
// is plain data, never a closure, so a clone copies it like cast/choosing.
//
// seats is the round order -- AliveFrom(0), a slice, deterministic, the same
// APNAP shape the engine uses everywhere. kept[i], taken[i] and seats[i]
// correspond by position. limit is Config.Mulligans (the free-mulligan count,
// rule R-8.4). bottom is false during the keep/mulligan phase and true during
// the bottoming phase; cursor names the next seat to ask in whichever phase.
type mulliganRound struct {
	seats  []state.PlayerID
	kept   []bool
	taken  []int
	limit  int
	bottom bool
	cursor int
}

// stepPregame issues the single next pregame decision -- one keep/mulligan
// ask, one bottoming ask, or the hand-off to beginTurn -- and returns. step()
// calls it exactly once per engine step and returns right after, so Advance's
// loop issues the round one decision at a time, each Submit answering the
// previous one; there is never more than one pregame decision pending.
func (e *Engine) stepPregame() {
	if e.G.Over {
		return
	}
	m := &e.mulligan
	if m.bottom {
		// Bottoming phase: each seat that mulliganed bottoms `taken` cards
		// from its kept hand (the London end-of-round bottoming). A seat that
		// never mulliganed (taken == 0) has nothing to bottom and is skipped.
		for m.cursor < len(m.seats) {
			i := m.cursor
			if m.taken[i] == 0 {
				m.cursor++
				continue
			}
			e.askBottoming(i)
			return
		}
		// Every seat that had something to bottom has bottomed: the round is
		// over and the first alive seat begins turn 1, exactly as before.
		e.pregame = false
		e.beginTurn(m.seats[0])
		return
	}
	// Keep/mulligan phase: ask each not-yet-kept seat, in round order,
	// whether to keep or (while it still has a free mulligan) mulligan.
	for m.cursor < len(m.seats) {
		i := m.cursor
		if m.kept[i] {
			m.cursor++
			continue
		}
		e.askKeepMulligan(i)
		return
	}
	// Every seat has kept: move to the bottoming phase.
	m.bottom = true
	m.cursor = 0
	e.stepPregame()
}

// askKeepMulligan offers seat i of the round a keep/mulligan decision. It is
// Min == Max == 1 over the same distinct-index shape Validate enforces
// everywhere (Ruling U2). While the seat still has a free mulligan
// (taken < limit) it offers both the "keep" and "mulligan" options; once it
// has used its whole allowance, London offers only "keep" -- you keep what
// you have.
// putCount is the number phrase for a London bottoming penalty: "1 card" or
// "2 cards" -- real English singular and plural, not "(s)" (finding bh). It
// is the human-facing count every mulligan prompt embeds.
func putCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return fmt.Sprintf("%d cards", n)
}

// bottomingPrompt is the human-readable wording for a London bottoming ask:
// `taken` cards leave the kept hand for the bottom of the library. It is the
// last real sentence a player reads in the mulligan round, so it is written
// for a human -- finding bh: no "(s)", no "bottoms" as a verb.
func bottomingPrompt(taken int) string {
	return fmt.Sprintf("Put %s on the bottom of your library", putCount(taken))
}

// keepMulliganPrompt is the human-readable wording for a keep/mulligan ask.
// It names the bottoming penalty a keep accepts, in the same real English as
// bottomingPrompt (finding bh: the old "keeps 7 and bottoms 1, or mulligans"
// was engine-speak). With a free mulligan remaining the seat has a choice;
// once the allowance is spent London offers only a keep.
func keepMulliganPrompt(taken, limit int) string {
	penalty := fmt.Sprintf("put %s on the bottom of your library", putCount(taken))
	if taken < limit {
		return fmt.Sprintf("Keep your hand (%s) or take a mulligan?", penalty)
	}
	return fmt.Sprintf("Keep your hand (%s)", penalty)
}

func (e *Engine) askKeepMulligan(i int) {
	m := &e.mulligan
	p := m.seats[i]
	opts := []decision.Option{{Index: 0, Kind: "keep", Label: "keep"}}
	if m.taken[i] < m.limit {
		opts = append(opts, decision.Option{Index: 1, Kind: "mulligan", Label: "mulligan"})
	}
	// CR 103.4: the seat re-drew a full openingHand on every mulligan, so
	// while it is deciding it always holds seven and will bottom `taken`
	// of them if it keeps -- the bottoming is the entire penalty.
	e.ask(decision.New(p, decision.KMulligan, keepMulliganPrompt(m.taken[i], m.limit), 1, 1, opts))
}

// askBottoming offers seat i a bottoming decision over its kept hand: one
// "bottom" option per card, Min == Max == taken[i] -- exactly the distinct-
// index shape Validate already enforces for KTriggerOrder (a bottoming choice
// is a permutation of taken[i] hand indices; Ruling U2).
func (e *Engine) askBottoming(i int) {
	m := &e.mulligan
	p := m.seats[i]
	hand := e.G.Zone(state.ZHand, p)
	opts := make([]decision.Option, len(hand))
	for j, id := range hand {
		opts[j] = decision.Option{Index: j, Kind: "bottom",
			Label: e.G.Obj(id).Face().Name, Obj: id, Player: p}
	}
	e.ask(decision.New(p, decision.KMulligan, bottomingPrompt(m.taken[i]), m.taken[i], m.taken[i], opts))
}

// handleMulligan applies a KMulligan answer. In the keep/mulligan phase (the
// round's first half, e.mulligan.bottom false) a keep marks the seat kept; a
// mulligan shuffles the seat's hand back into its library (Shuffle, secret),
// takes a free mulligan and draws a full new hand of seven -- CR 103.4: the
// bottoming of `taken` cards at the round's end is the entire penalty, never
// a smaller redraw. Every mutation is an e.emit or drawCard, so the whole
// round is event-driven and replays byte-for-byte. In the bottoming phase it
// moves each chosen card to its library bottom and advances the round past
// this seat.
func (e *Engine) handleMulligan(d *decision.Decision, in decision.Intent) {
	if e.mulligan.bottom {
		e.handleBottoming(d, in)
		return
	}
	// Resolve the round index from the answering seat rather than the
	// cursor (Finding 3, fix round 1): d.Player is the authority -- Submit
	// has already rejected any answer whose Player is not the pending
	// decision's, and askKeepMulligan addressed that decision to
	// seats[cursor] -- so the two always agree. Deriving the index from the
	// seat keeps the seats slice the round's single mapping from seat to
	// kept/taken bucket and removes the cursor coupling for free.
	i := -1
	for j, s := range e.mulligan.seats {
		if s == d.Player {
			i = j
			break
		}
	}
	p := d.Player
	chosen := d.Chosen(in)
	if len(chosen) > 0 && chosen[0].Kind == "keep" {
		e.mulligan.kept[i] = true
		return
	}
	// A mulligan: the seat stays un-kept (it must decide again, on a full
	// re-drawn seven), so cursor does not advance. taken increments first;
	// once it reaches limit the only follow-up ask offers "keep".
	e.mulligan.taken[i]++
	for _, id := range e.G.Zone(state.ZHand, p) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand,
			To: state.ZLibrary, Player: p, Text: "mulligan"})
	}
	order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
	e.rng.Shuffle(order)
	e.emit(events.Event{Kind: events.Shuffle, Player: p, IDs: order, Secret: true})
	// CR 103.4: a mulligan draws a FULL new hand of seven -- the later
	// bottoming is the entire penalty, and the redraw is literally the same
	// loop genesis already runs in New's opening deal. If the seat decks out
	// mid-redraw (drawCard's own checkStateBased sets Over; reachable only
	// from a hand-made tiny deck) stop drawing: the round respects Over
	// everywhere else and must here too.
	for j := 0; j < openingHand && !e.G.Over; j++ {
		e.drawCard(p)
	}
}

// handleBottoming moves each card the bottoming answer chose to the bottom
// of its owner's library -- a library's bottom is its last element (Move
// appends to the destination zone's end) -- in the order the client
// submitted them, then advances the round past this seat.
func (e *Engine) handleBottoming(d *decision.Decision, in decision.Intent) {
	e.mulligan.cursor++
	for _, o := range d.Chosen(in) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.Obj, From: state.ZHand,
			To: state.ZLibrary, Player: d.Player, Text: "bottomed"})
	}
}
