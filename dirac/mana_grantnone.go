package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A plain {T} mana ability of no colour (Command Tower, Path of Ancestry, Arcane
// Signet: "Add one mana of any color in your commander's color identity", which
// adds nothing without a commander, CR 903.4f) beside a granted mana ability
// (Cryptolith Rite): gorge's census does not cover a Produced$ ColorIdentity
// ability, so it is a gap, and its one prefix search (potential_script.go
// scriptSearch.visit) activates every census gap in battlefield order, this one
// among the grants. The tap adds no mana but changes the payer state, so the
// found prefix carries it. The kernel's planner profile drops such a source
// (gen.go), so the grant search lists it as a gap candidate itself (CR 605.3).

// stepManaNone is the gi of a prefix step that taps a source of no colour.
const stepManaNone int16 = -5

// manaNoneAb reports whether a is a plain {T} mana ability of no colour: no
// chosen, dynamic, reflected, each-colour or choice production, no spend
// restriction and no deferred rider.
func manaNoneAb(a *kop.Ability) bool {
	return a.ManaColors == 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags&(kop.AfManaChosen|kop.AfManaDynamic|kop.AfLatent|kop.AfManaEachColor|kop.AfManaChoice|kop.AfManaReflect) == 0 &&
		a.ManaRestrict == 0 && a.ManaAmong == 0 && !a.ManaDeferred
}

// scriptManaNone: p's activatable sources of no colour, a slot mask.
func (s *State) scriptManaNone(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.anyManaNone {
		return slotMask{}
	}
	return s.gapMask(k, p, gfManaNone)
}

// noneBranch taps the no-colour candidate i (slot sl) when it is still untapped:
// a dead-end step that adds no mana, so no ask node.
func (g *grantSrch) noneBranch(t *State, i int, sl Slot) bool {
	if t.perms[sl].flags&pfTapped != 0 {
		return false
	}
	return g.branch(t, i, sl, stepManaNone, 0, 0, false)
}

// noneApply taps the no-colour source of a found prefix step on s; no mana
// floats, only the tap's extra the search priced (branch; mana_tapextra.go).
func (s *State) noneApply(k *prep, p uint8, sl Slot, col uint8) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, s.stepView(k, sl, stepManaNone), int(col))
		s.addXv(p&1, &xv)
	}
}
