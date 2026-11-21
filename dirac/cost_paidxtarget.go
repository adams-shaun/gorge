package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// paidXTargetAbort reports whether a pick cost of the entry being built
// compares against the announced {X} and no longer has enough candidates at
// the X that was answered.
//
// CR 601.2b announces X before CR 601.2c chooses targets, and CR 601.2h pays
// the cost. Gorge's target menu (affordableTargetCandidates) reprices the
// announced cost for each candidate, so a paid-X part -- Knollspine
// Invocation's Discard<1/Card.cmcEQX>, SVar:X:Count$xPaid -- whose matching
// hand cards are gone withholds every target and aborts the proposal ("cast
// aborted: no legal target"). The kernel used to ask the target and only
// discover the short cost at bsCosts, reversing one decision later with the
// ability still on the stack. The X ask still offers every X the mana can
// pay, as gorge's does; the filter's value at the announced X is the target
// stage's to judge.
func (s *State) paidXTargetAbort(k *prep) bool {
	e := &s.bld().e
	if e.kind != seSpell && e.kind != seAbility {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil || !costFilterBoundByX(k.t, pc) {
		return false
	}
	return !s.pickCostPayable(k, e.ctrl, pc, e.obj)
}

// costFilterBoundByX reports whether a filter of a pick part compares against
// the X the cost announces (PredV VX). kgen refuses a target-bound SVar X read
// in a cost ("Targeted$CardManaCost"), so a VX bound in a cost filter is the
// announced {X}.
func costFilterBoundByX(t *kop.Table, c *kop.Cost) bool {
	for _, f := range [...]kop.FilterID{c.Filter, c.Discard2F, c.DiscardSideF} {
		for ; f != 0; f = t.Filters[f].Or {
			if t.Filters[f].PredV.K == kop.VX {
				return true
			}
		}
	}
	return false
}
