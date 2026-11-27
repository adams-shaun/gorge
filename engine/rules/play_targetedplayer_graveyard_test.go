package rules

// api:Play Valid$ Card.TargetedPlayerCtrl ValidZone$ Graveyard (Sorcerous
// Squall's FishyCast): after the mill moves the targeted player's cards, the
// resolving controller is offered that player's graveyard instants and
// sorceries -- cards they own, offered to a controller who does not. A
// ReplaceGraveyard$ Exile rider then keeps a spell cast this way out of the
// graveyard.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayTargetedPlayerGraveyardOffersMilledCard casts Sorcerous Squall,
// answers its target and mill, and asserts the play ask offers the targeted
// player's milled instant -- and not the milled creature (the ValidSA$
// gate) -- then casts it and pins the exile-instead-of-graveyard rider.
func TestPlayTargetedPlayerGraveyardOffersMilledCard(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusCard(t, "Sorcerous Squall"))
	squall := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(squall); o == nil || o.Zone != state.ZHand || o.Owner != 0 {
		t.Fatalf("setup: squall = %+v, want seat 0's hand", o)
	}

	// Seat 1's library top: the instant the mill must move, and a creature
	// the ValidSA$ Instant,Sorcery filter must keep out of the offer.
	surge := e.G.AddObject(card(t, "Name:Test Surge\nManaCost:U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"), 1)
	bear := e.G.AddObject(card(t, "Name:Test Bear\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	surge.Zone = state.ZLibrary
	bear.Zone = state.ZLibrary
	e.G.SetZone(state.ZLibrary, 1, append([]state.ObjID{surge.ID, bear.ID}, e.G.Zone(state.ZLibrary, 1)...))
	if o := e.G.Obj(surge.ID); o == nil || o.Zone != state.ZLibrary || o.Owner != 1 {
		t.Fatalf("setup: surge = %+v, want seat 1's library", o)
	}

	addMana(t, e, 0, "UUU666666")
	castIdx := -1
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == squall {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("squall cast not offered: %+v", e.legalActions(0))
	}
	submitChoices(t, e, castIdx)

	// The target ask: answer it with seat 1, the opponent.
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("target ask missing: %+v", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Player == 1 && o.Obj == 0 {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("no seat 1 player-target option: %+v", d.Options)
	}
	submitChoices(t, e, tgt)

	// Drain to the mid-resolution play ask.
	ask := passPriorityUntil(t, e, decision.KModes)
	if ask.ResumeKind != "play" {
		t.Fatalf("pending KModes is not the play ask: %+v", ask)
	}

	// Precondition the offer depends on: the mill moved both cards into seat
	// 1's graveyard (they were in its library above, not the graveyard).
	if o := e.G.Obj(surge.ID); o == nil || o.Zone != state.ZGraveyard || o.Owner != 1 {
		t.Fatalf("milled surge = %+v, want seat 1's graveyard", o)
	}
	if o := e.G.Obj(bear.ID); o == nil || o.Zone != state.ZGraveyard || o.Owner != 1 {
		t.Fatalf("milled bear = %+v, want seat 1's graveyard", o)
	}

	offered := map[state.ObjID]bool{}
	for _, o := range ask.Options {
		if o.Kind == "mode" {
			offered[o.Obj] = true
		}
	}
	if !offered[surge.ID] {
		t.Fatalf("targeted player's milled instant not offered: %+v", ask.Options)
	}
	if offered[bear.ID] {
		t.Fatalf("milled creature offered through ValidSA$ Instant,Sorcery: %+v", ask.Options)
	}
	if ask.Min != 0 {
		t.Fatalf("Optional$ True play ask has Min %d, want 0", ask.Min)
	}

	// Cast the offered instant: it resolves and, per ReplaceGraveyard$
	// Exile, sits in exile rather than the graveyard it was offered from.
	pick := -1
	for _, o := range ask.Options {
		if o.Kind == "mode" && o.Obj == surge.ID {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)
	if o := e.G.Obj(surge.ID); o == nil || o.Zone != state.ZStack {
		t.Fatalf("offered instant did not begin its cast: %+v", o)
	}
	passUntilStackEmpty(t, e, 50)
	if o := e.G.Obj(surge.ID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("ReplaceGraveyard$ Exile: cast instant zone = %v, want exile", e.G.Obj(surge.ID).Zone)
	}
}
