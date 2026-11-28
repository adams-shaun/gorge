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
	s.eachStaticG(k, func(stc statSrc) bool {
		a := stc.a
		if !raiseTgtStatic(a) {
			return true
		}
		st := &a.Static
		if f := st.StFlags; f&kop.StRaiseYou != 0 && stc.ctrl != p || f&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		ctx := s.ctxStatic(stc)
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) || !s.matchSpellCard(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl, stc.emb) {
			return true
		}
		hit = true
		return false
	})
	return hit
}

// windowPoolPipsFirst is the second path targetWindowShort tries when the
// window's battlefield-order walk falls short: the floating pool pays its own
// pips of cost first (held aside, so a filter source's {1} fee cannot spend a
// unit a pip needs), then the window pays the rest. gorge's
// affordableTargetCandidates asks castWindowReachable, which finds this order
// whenever it exists: Might of Alara's floating {G} under Icefall Regent's
// {2} tax, with a Mountain and a Darkwater Catacombs ({1},{T}: add {U}{B}),
// pays {2}{G} though the walk taps the Catacombs first and spends the {G} on
// its fee. A hybrid, snow, X or any-colour cost keeps the walk's answer.
func (s *State) windowPoolPipsFirst(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) bool {
	if anyColor || cost.X != 0 || cost.Snow != 0 || cost.Hyb[0] != 0 {
		return false
	}
	rest := *cost
	var held [kop.NumManaKinds]uint8
	pool := &s.pl[p].pool
	for c := range rest.Pips {
		held[c] = min(rest.Pips[c], pool[c])
		rest.Pips[c] -= held[c]
		pool[c] -= held[c]
	}
	ok, _ := s.windowTapRider(k, p, &rest, false, false)
	return ok
}

// windowOwedFirst is targetWindowShort's third path: the window walked with
// each source answering what the cost still owes (the clamp) and the paid-fee
// filters left out of p's sources. The walks before it answer a source its
// first colour and let a filter spend a unit a pip needs: Skycloud Expanse
// ({1}, {T}: add {W}{U}), first in order, takes Mystic Monastery's mana as its
// fee, and an improvise cast still owing {R} reads unpayable though the
// Monastery, a Plains and a Temple pay it (CR 601.2g-h, 702.126a). gorge's
// castWindowReachable searches every set and order of activations, so one path
// that pays settles it. It moves p's control mask: the caller restores the
// state.
func (s *State) windowOwedFirst(k *prep, p uint8, cost *kop.ManaCost) bool {
	si := SeatIx(p)
	s.ctrlM[si] = s.ctrlM[si].andNot(s.scriptFilt(k, p))
	ok, _ := s.windowTapRider(k, p, cost, true, false)
	return ok
}
