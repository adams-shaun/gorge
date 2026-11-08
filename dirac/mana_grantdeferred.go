package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A deferred mana source (Pyromancer's Goggles' TriggersWhenSpent$ {R},
// mana_deferred.go) beside a granted mana ability (New Horizons on a land):
// gorge's census lists neither (pay.PlanHasSpecialProductionParam defers the
// one, PaymentAbility excludes the other), and its one prefix search
// (potential_script.go potentialPrefixScript) activates every census gap in
// battlefield order, the deferred source among the grants. So the grant
// search (grantSrch) lists the deferred source as a gap candidate too: a
// Mountain under New Horizons for {R}{R} plus Goggles' {R} and three printed
// lands pay Lathliss, Dragon Queen's {4}{R}{R} (CR 601.2g-h, 605.1a).

// stepDeferred is the gi of a prefix step that taps a deferred source for its
// {R}.
const stepDeferred int16 = -4

// deferBranch taps the deferred candidate i (slot sl) for its {R} when it is
// still untapped: a single-ability source, so no ask node.
func (g *grantSrch) deferBranch(t *State, i int, sl Slot) bool {
	if t.perms[sl].flags&pfTapped != 0 {
		return false
	}
	return g.branch(t, i, sl, stepDeferred, 1<<kop.R, 1, false)
}

// deferApply taps the deferred source of a found prefix step on s: its {R}
// floats as the deferred source's (floatDeferred), so a cast that spends it is
// paid by Goggles and its rider can fire.
func (s *State) deferApply(k *prep, p uint8, sl Slot) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.floatDeferred(k, p, sl)
}
