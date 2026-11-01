package dirac

import (
	"testing"
)

// TestChangelingOutsideType: a printed changeling (CardDef.AllCreatureTypes)
// has a chosen creature type outside the matchup's subtypes (gorge
// IntrinsicAllCreatureTypes), as an all-creature-types override does.
func TestChangelingOutsideType(t *testing.T) {
	sc, a, b := kwEvasionScen(t)
	c := sc.s.objs[a].card
	if sc.s.hasSubGen(sc.k, sc.slot(a), subOutside) {
		t.Fatal("a plain creature has an outside creature type")
	}
	sc.k.t.Cards[c].AllCreatureTypes = true
	if !sc.s.hasSubGen(sc.k, sc.slot(a), subOutside) || !sc.s.hasSubGen(sc.k, sc.slot(b), subOutside) {
		t.Fatal("a printed changeling lacks an outside creature type")
	}
}
