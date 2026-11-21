// Cursecloth Wrappings' pump grants "Embalm:CardManaCost" to a graveyard
// creature card (CR 702.128a): the embalm cost is the card's own mana cost.
// The synthesized body's Cost$ must carry that cost, not the CardManaCost
// token ParseCost reads as generic 1.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestGrantedEmbalmCardManaCostPrice(t *testing.T) {
	t.Parallel()
	const bear = "Name:Granted Embalm Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := handEngine(t)
	id := graveyardCard(t, e, bear)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear = %+v, want a graveyard card", o)
	}
	if o.Face().HasKeyword("Embalm") {
		t.Fatal("precondition: the bear must print no Embalm, else the offer is not the granted one")
	}
	o.IntrinsicKeywords = append(o.IntrinsicKeywords, "Embalm:CardManaCost")
	e.staticEpoch, e.activeEpoch = -1, -1
	if got := e.grantedKeywordLines(id); len(got) != 1 || got[0] != "Embalm:CardManaCost" {
		t.Fatalf("precondition: granted keyword lines = %q, want [Embalm:CardManaCost]", got)
	}

	// The ability body prices at the card's mana cost plus the self-exile.
	ab := grantedKeywordSA(o, "Embalm:CardManaCost")
	if ab == nil {
		t.Fatal("grantedKeywordSA returned nil for a card with a mana cost")
	}
	c := e.abilityCompiledCost(ab).Cost
	if c.Generic != 1 || c.Colored.Total() != 1 || len(c.Unknown) != 0 || len(c.Exile) != 1 {
		t.Fatalf("granted embalm cost = %+v, want {1}{G} plus exiling the card", c)
	}

	// {1} alone no longer buys the embalm.
	addMana(t, e, 0, "C")
	if _, ok := grantedCyclingOption(e, 0, id); ok {
		t.Fatal("granted embalm offered with only {1} available; its cost is {1}{G}")
	}
	addMana(t, e, 0, "G")
	opt, ok := grantedCyclingOption(e, 0, id)
	if !ok {
		t.Fatal("granted embalm not offered with {1}{G} available")
	}
	e.beginActivation(0, opt)
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after paying the granted embalm = %d, want 0 ({1}{G} charged)", got)
	}
	if got := e.G.Obj(id).Zone; got != state.ZExile {
		t.Fatalf("embalm source in %s, want exile (cost exiles it)", got)
	}
}
