package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

func TestForetoldPredicateMatchesFlaggedExile(t *testing.T) {
	g := state.NewGame([]string{"A"})
	o := g.AddObject(card(t, "Name:Foretold\nManaCost:3\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	o.Zone = state.ZExile
	o.CastFlags |= state.FlagForetold
	if !effects.MatchesSpecCtx(g, "Card.foretold+YouOwn", o.ID, effects.SpecContext{}) {
		t.Fatal("foretold object predicate did not match a flagged exiled card")
	}
	o.CastFlags = 0
	if effects.MatchesSpecCtx(g, "Card.foretold+YouOwn", o.ID, effects.SpecContext{}) {
		t.Fatal("foretold object predicate matched an unflagged card")
	}
}

func TestForetoldCostUsesPrintedManaMinusTwo(t *testing.T) {
	f := card(t, "Name:Marked\nManaCost:4 W\nTypes:Creature\nPT:2/2\nOracle:x\n").Faces[0]
	got, ok := foretellCost(f)
	if !ok {
		t.Fatal("printed-cost ForetoldCost shape was not priceable")
	}
	if got.Generic != 2 || got.Colored[state.MW] != 1 {
		t.Fatalf("foretell cost = %+v, want {2}{W}", got)
	}
}
