package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// castModeOptions lists the Modes of id's cast options in opts.
func castModeOptions(opts []decision.Option, id state.ObjID) (modes []string) {
	for _, o := range opts {
		if o.Kind == "cast" && o.Obj == id {
			modes = append(modes, o.Mode)
		}
	}
	return modes
}

// Buyback (CR 702.27a) is offered beside the plain cast once the pool pays
// the base cost plus the buyback cost.
func TestBuybackOfferedBesideThePlainCast(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Searing Touch"))
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MR] = 1
	if got := castModeOptions(e.legalActions(0), spell); len(got) != 1 || got[0] != "" {
		t.Fatalf("with {R} only: cast modes %q, want just the plain cast", got)
	}
	e.G.Players[0].Pool[state.MC] = 4
	if got := castModeOptions(e.legalActions(0), spell); len(got) != 2 || got[0] != "" || got[1] != "buyback" {
		t.Fatalf("with {R}{4}: cast modes %q, want plain and buyback", got)
	}
}

// The buyback cast announces the same targets the plain cast does (CR
// 601.2c), so it is not offered while the spell has no legal target: Elvish
// Fury with no creature on the battlefield has neither cast.
func TestBuybackNotOfferedWithoutATarget(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Elvish Fury"))
	spell := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 4
	if got := castModeOptions(e.legalActions(0), spell); len(got) != 0 {
		t.Fatalf("no creature to target: cast modes %q, want none", got)
	}
	target := e.G.AddObject(card(t, "Name:Target\nTypes:Creature Human\nPT:1/1\nOracle:x\n"), 1)
	target.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{target.ID})
	if got := castModeOptions(e.legalActions(0), spell); len(got) != 2 || got[1] != "buyback" {
		t.Fatalf("a creature to target: cast modes %q, want plain and buyback", got)
	}
}
