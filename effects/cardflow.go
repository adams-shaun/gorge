package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("Draw", effDraw)
	Register("Discard", effDiscard)
	Register("Mill", effMill)
	Register("Dig", effDig)
	Register("Reveal", effReveal)
	Register("RevealHand", effReveal)
	Register("PeekAndReveal", effReveal)
	Register("RearrangeTopOfLibrary", effRearrangeTopOfLibrary)
	Register("NameCard", effNameCard)
}

// DrawFor is exported so the rules package can use the same code path for the
// draw step. Drawing from an empty library is a loss, checked by SBAs.
func DrawFor(h Host, p state.PlayerID) {
	g := h.Game()
	lib := g.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		h.Emit(events.Event{Kind: events.PlayerLost, Player: p, Text: "drew from an empty library"})
		return
	}
	h.Emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
		From: state.ZLibrary, To: state.ZHand, Secret: true})
}

func effDraw(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			DrawFor(h, p)
		}
	}
}

// effDiscard discards from the top of hand order. Real discard is a choice;
// M1's decks discard only to the cleanup step and to Delve-style costs, where
// "first in hand" is deterministic and adequate. Task 20's choice plumbing is
// where a player-chosen discard would hook in.
func effDiscard(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			hand := g.Zone(state.ZHand, p)
			if len(hand) == 0 {
				break
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: hand[0],
				From: state.ZHand, To: state.ZGraveyard, Player: p})
		}
	}
}

// effMill moves cards from the top of a player's library straight to their
// graveyard -- Discard's sibling, minus the hand.
func effMill(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		for i := int32(0); i < n; i++ {
			lib := g.Zone(state.ZLibrary, p)
			if len(lib) == 0 {
				break
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: lib[0],
				From: state.ZLibrary, To: state.ZGraveyard, Player: p})
		}
	}
}

// effDig is M1's simplification of Forge's Dig: look at the top DigNum cards
// of Defined$'s library, move up to ChangeNum of the ones matching
// ChangeValid$ (default "Card") to DestinationZone$ (default "Hand"), and
// leave everything else exactly where it already is -- on top of the
// library, in its existing relative order. The brief's own spec names the
// remainder's destination "LibraryPosition2$", but that parameter does not
// exist anywhere in the fetched corpus (the real field there is
// "LibraryPosition$", also left unhandled here); M1 keeps the existing order
// for the untaken cards either way, the same simplification
// RearrangeTopOfLibrary makes for its own remainder.
//
// A real card can also write "ChangeNum$ All" (e.g. Goblin Guide's own Dig)
// instead of a number. Num has no notion of "All" and falls back to its
// zero default, so today that degrades to "look, but move nothing" rather
// than moving every matching card -- a documented, non-crashing degradation
// of the same kind as the Repeat/MaxRepeat one below, not a special case
// this function papers over.
func effDig(h Host, c *Ctx, sa *cards.SA) {
	digNum := Num(h, c, sa, "DigNum", 1)
	if digNum < 0 {
		digNum = 0
	}
	changeNum := Num(h, c, sa, "ChangeNum", digNum)
	if changeNum < 0 {
		changeNum = 0
	}
	spec := sa.Params["ChangeValid"]
	if spec == "" {
		spec = "Card"
	}
	destName := sa.Params["DestinationZone"]
	if destName == "" {
		destName = "Hand"
	}
	dest := ParseZone(destName)
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := g.Zone(state.ZLibrary, p)
		n := digNum
		if int32(len(lib)) < n {
			n = int32(len(lib))
		}
		top := append([]state.ObjID(nil), lib[:n]...)
		moved := int32(0)
		for _, id := range top {
			if moved >= changeNum {
				break
			}
			if !MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				continue
			}
			h.Emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: dest, Secret: true})
			moved++
		}
	}
}

// effReveal backs Reveal, RevealHand and PeekAndReveal, which the brief
// specifies as one row sharing a single Amount param (NumCards, default 1)
// and one behaviour: reveal cards without disturbing them, recorded via a
// non-Secret Note carrying their identities so view projection stops
// redacting them. PeekAndReveal looks at the library; Reveal and RevealHand
// look at hand. (RevealHand's real corpus params reveal the whole hand
// rather than a count; M1 follows the brief's shared NumCards spec for all
// three instead of special-casing that.)
func effReveal(h Host, c *Ctx, sa *cards.SA) {
	amt := Num(h, c, sa, "NumCards", 1)
	if amt < 0 {
		amt = 0
	}
	zone := state.ZHand
	if sa.API == "PeekAndReveal" {
		zone = state.ZLibrary
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		pool := g.Zone(zone, p)
		n := amt
		if int32(len(pool)) < n {
			n = int32(len(pool))
		}
		if n == 0 {
			continue
		}
		h.Emit(events.Event{Kind: events.Note, Player: p, Text: "reveals cards",
			IDs: append([]state.ObjID(nil), pool[:n]...)})
	}
}

// effRearrangeTopOfLibrary looks at the top NumCards of Defined$'s library.
// M1 keeps the existing order -- the choice of a new order is Task 20's
// territory -- so the only observable effect is the Note recording what was
// seen.
func effRearrangeTopOfLibrary(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumCards", 1)
	if n < 0 {
		n = 0
	}
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		p := PlayerOf(h, c, t)
		lib := g.Zone(state.ZLibrary, p)
		k := n
		if int32(len(lib)) < k {
			k = int32(len(lib))
		}
		h.Emit(events.Event{Kind: events.Note, Player: p,
			Text: "looks at the top of the library, order unchanged",
			IDs:  append([]state.ObjID(nil), lib[:k]...)})
	}
}

// effNameCard is M1's simplification of a real naming choice (Task 20's
// territory): it names the first card in the controller's library, which is
// at least a deterministic, legal name for whatever downstream sub-ability
// expects one.
func effNameCard(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	lib := g.Zone(state.ZLibrary, c.Controller)
	name := ""
	if len(lib) > 0 {
		if o := g.Obj(lib[0]); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
	}
	h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "names " + name})
}
