package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Cascade (CR 702.85, task cascade1). The keyword is implemented as a real
// triggered ability: rules' cast flow queues one pendingTrigger per Cascade
// instance (printed K:Cascade line or layer-6 grant), the drain mints a
// respondable KeywordTriggerPush stack object whose "__kwCascade" payload
// events.Apply rebuilds into the DB$ Cascade body registered here, and the
// ability's RESOLUTION runs the exile-until + may-cast sequence.
//
// CR 702.85a, as implemented:
//   - "exile cards from the top of your library until you exile a nonland
//     card that costs less" — the scan walks the caster's library in zone
//     order (index 0 = top), exiling each card with a logged MoveZone (the
//     exile is public: the Note after the loop names every exiled id, the
//     same non-Secret ids-Note shape effDigUntil's reveal emits), and stops
//     at the first nonland card whose mana value is STRICTLY less than the
//     cascade spell's. The found card itself is exiled too — a later free
//     cast is cast FROM EXILE, exactly as the oracle says.
//   - "You may cast it without paying its mana cost" — a real optional
//     election through the effPlay shape (DB$ Play | WithoutManaCost$ True |
//     Optional$ True, population = the found card), whose answer rules'
//     resumeResolution "play" arm turns into a zero-mana-cost cast from
//     exile. The population rides the resolution's Remembered.
//   - "Put the exiled cards on the bottom in a random order" — the random
//     order is the engine's standing determinism stand-in (the same one the
//     Dig rows record in AGENTS.md): the cards return in their existing
//     library order. The cards that did not match are bottomed right after
//     the scan (before the election — nothing between the election and that
//     bottoming can observe the difference, and the found card would be the
//     last move either way); the found card is bottomed by the chained
//     CascadeBottom sub-ability when it was NOT cast (it is still in exile),
//     and is a no-op when it was (it is on the stack).
//
// The cascade spell's mana value is the PRINTED value of its face read at
// resolution (CR 202.3): no corpus K:Cascade carrier carries {X} (measured),
// so the X-in-cost caveat the stack-object mana value would need is
// corpus-unreachable. The candidate's mana value is likewise its printed
// face value, with {X} counted as 0 (CR 202.3b) — the same read every other
// mana-value comparison in the engine makes off a Face.
//
// A library with no qualifying card exiles everything to the last card and
// bottoms it all with no election (the scan simply runs out).
func init() {
	Register("Cascade", effCascade)
	Register("CascadeBottom", effCascadeBottom)
}

// cascadePlaySA builds the free-cast election's Play SA and its chained
// CascadeBottom tail. Hand-built (the cascade trigger's body is synthesized
// from the keyword, so no face SVar table carries it) and freshly built per
// call: SA values are immutable shared card data elsewhere, but this one is
// per-resolution scratch, built here and referenced only by the resolution
// it is built for (the pending frame's ResumeSA and the continuation chain
// both point at it, never at a compiled face).
func cascadePlaySA() *cards.SA {
	play := &cards.SA{Kind: "DB", API: "Play", Params: map[string]string{
		"Defined":            "Remembered",
		"WithoutManaCost":    "True",
		"Optional":           "True",
		"TriggerDescription": "Cascade",
	}}
	play.Sub = &cards.SA{Kind: "DB", API: "CascadeBottom", Params: map[string]string{}}
	return play
}

// effCascade runs one cascade trigger's exile-until + may-cast sequence.
// Source is the cascade spell (the trigger's Source — the stack object the
// cast pushed), Controller the caster; the library exiled from is the
// controller's own.
func effCascade(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	src := g.Obj(c.Source)
	if src == nil || src.Face() == nil {
		return
	}
	p := c.Controller
	if int(p) >= len(g.Players) || g.Players[p].Lost {
		return
	}
	mv := int(src.Face().Cmc())
	// Snapshot the library before moving out of it: the MoveZone emissions
	// mutate the zone slice under us.
	lib := append([]state.ObjID(nil), g.Zone(state.ZLibrary, p)...)
	var exiled []state.ObjID
	found := state.ObjID(0)
	for _, id := range lib {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary,
			To: state.ZExile, Text: "cascaded"})
		exiled = append(exiled, id)
		if !o.Face().IsLand() && int(o.Face().Cmc()) < mv {
			found = id
			break
		}
	}
	if len(exiled) > 0 {
		h.Emit(events.Event{Kind: events.Note, Player: p, IDs: exiled,
			Text: "cascades, exiling cards from the top of the library"})
	}
	// Bottom the cards the scan exiled below the found one now, in their
	// existing library order (the random-order stand-in). The found card is
	// by construction the LAST card the scan exiled, so the final library
	// order is the same one a single post-election bottoming would produce.
	rest := exiled
	if found != 0 {
		rest = exiled[:len(exiled)-1]
	}
	for _, id := range rest {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile,
			To: state.ZLibrary, Text: "cascaded card put on the bottom of the library"})
	}
	if found == 0 {
		return
	}
	// The election: population = exactly the found card, riding the
	// resolution's Remembered (effPlay's Defined$ Remembered read and the
	// answer's ResumeRemembered ride are the same list). The chained
	// CascadeBottom bottoms the found card when the election declined it.
	c.Remembered = []state.Target{{Obj: found}}
	playSA := cascadePlaySA()
	effPlay(h, c, playSA)
	if !h.Suspended() {
		// AskNoHost (an effects-package stub host): the deterministic decline
		// (R-9). The real engine's Ask always suspends, and the answer
		// re-enters playSA, whose Sub chain runs CascadeBottom there.
		effCascadeBottom(h, c, playSA.Sub)
	}
}

// effCascadeBottom is the election's chained tail: every card the cascade
// exiled that is STILL in exile (the found card when it was not cast) goes
// to the bottom of its owner's library. A card the free cast took has
// already left exile for the stack and is skipped, which is what makes the
// same tail correct on the accept and the decline arm both.
func effCascadeBottom(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	remembered := c.Remembered
	c.Remembered = nil
	for _, t := range remembered {
		if t.IsPlayer || t.Obj == 0 {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZExile {
			continue
		}
		h.Emit(events.Event{Kind: events.MoveZone, Obj: t.Obj, From: state.ZExile,
			To: state.ZLibrary, Text: "the uncast cascade card is put on the bottom of its owner's library"})
	}
}
