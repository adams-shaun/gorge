package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A battlefield RaiseCost static with a ValidTarget$ that raises the generic
// mana of a spell whose target it names (Kasmina, Enigmatic Mentor: "spells
// your opponents cast that target a creature or planeswalker you control cost
// {2} more"). raiseCostGen charges it once the cast's targets are chosen; gorge's
// cast planner drops the key and declines the whole cast's witness
// (paymentPlanTargetDependentFor: "shape:target_dependent_cost"), so on the
// autopay surface the cast is offered only when the floating pool pays it, and
// the lowering surface prices it from the composed cost like the card's own
// target-conditional reduction. Cold code: every site is behind
// genPrep.anyRaiseTgt.

// tableRaiseTgt reports whether t has a battlefield generic raise gated by a
// ValidTarget$ (a life raise is boardLifeTgtDep's, the card's own and the
// stack-source raise are priced by their own walks).
func tableRaiseTgt(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; raiseTgtStatic(a) {
				return true
			}
		}
	}
	return false
}

// raiseTgtStatic: a is the static tableRaiseTgt looks for.
func raiseTgtStatic(a *kop.Ability) bool {
	st := &a.Static
	return a.Kind == kop.AbStatic && st.Kind == kop.StatIncreaseCost && st.StFlags&kop.StTargetsFilter2 != 0 &&
		st.StFlags&kop.StRaiseLife == 0 && st.Zone != kop.ZoneCommand && st.Zone != kop.ZoneStack
}

// boardRaiseTgtDep reports that p's cast of card c (object o) is priced by a
// target through a battlefield generic raise: the spell announces a target
// and some such static applies to it whichever target it chooses (gorge
// paymentPlanTargetDependentFor drops ValidTarget$ and asks whether the static
// still applies). It is spellTgtDep's and plannedCastCostOf's battlefield half
// beside boardLifeTgtDep.
func (s *State) boardRaiseTgtDep(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen == nil || !k.gen.anyRaiseTgt || tspec(k, c, spellAb(k, c), 0) == nil {
		return false
	}
	hit := false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if !raiseTgtStatic(a) {
			return true
		}
		st := &a.Static
		if f := st.StFlags; f&kop.StRaiseYou != 0 && stc.ctrl != p || f&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		ctx := s.ctxStatic(stc)
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) || !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl) {
			return true
		}
		hit = true
		return false
	})
	return hit
}
