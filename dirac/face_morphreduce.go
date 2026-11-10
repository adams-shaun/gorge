package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A face-down keyword's printed reduction ("Disguise {5}{R}. This cost is
// reduced by {1} for each instant and sorcery card in your graveyard"):
// gorge (rules morph_turnup.go morphTurnUpMods) folds the amount into the
// turn-up price as a generic reduction, floored at zero, at the offer, the
// re-proof and the settlement alike (CR 601.2f, 702.168c). The kernel prices
// the keyword's turn-up through morphCostAt everywhere it reads MorphCost.

// morphCostAt is the face-down permanent at sl's keyword turn-up cost:
// MorphCost less CardDef.MorphReduce, evaluated for the permanent's
// controller.
func (s *State) morphCostAt(k *prep, sl Slot) kop.ManaCost {
	cd := &k.t.Cards[s.cardAt(sl)]
	c := cd.MorphCost
	if cd.MorphReduce == (kop.ValueRef{}) || DebugNoMorphReduce {
		return c
	}
	pm := &s.perms[sl]
	n := s.evalV(k, cd.MorphReduce, evalCtx{src: pm.obj, srcSl: sl, ctrl: pm.ctrl})
	if n <= 0 {
		return c
	}
	c.Generic = uint8(max(0, int(c.Generic)-n))
	return c
}

// DebugNoMorphReduce drops the reduction (the lockstep test's mutation).
var DebugNoMorphReduce bool
