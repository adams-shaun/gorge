package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The fee-order fallback (gorge potentialPrefixScript's second build with
// scriptSearch.feeOrder). When the plain run of a plain cast's prefix search
// spends its whole node budget without a script, gorge runs it again with the
// covered funding first, then the census gaps whose mana ability costs mana
// and makes more than one unit (Baxter Building's {4},{T}: four mana in any
// combination), then the other gaps: the fee is paid from mana already
// floating (CR 601.2b, 605.3b), so floating it first reaches the funding line
// the plain order's twin branches spent the budget on. A one-unit fee source
// is a filter whose net production is zero, which the plain order prices.

// filtFeeGaps is the filters of f gorge counts as fee gaps: a fee after cost
// reductions, and a fixed amount of at least two units (feeGaps in
// potentialPrefixScript; a dynamic amount counts none).
func (s *State) filtFeeGaps(k *prep, f slotMask) slotMask {
	var m slotMask
	for ; f.any(); f = f.rest() {
		sl := f.first()
		if s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
			m = m.with(sl)
		}
	}
	return m
}

// grantFeeGap reports whether the census gap at sl carries gorge's fee order:
// a mana ability of the source, printed or granted, that costs mana and makes
// at least two units (potentialPrefixScript's feeGaps scan: the units are the
// amount times a fixed production's colour total, so an any-colour one counts
// its amount alone; a dynamic amount counts none, as filtFeeGaps holds). A
// one-unit fee source is a filter whose net production is zero, which the
// plain order prices.
func (s *State) grantFeeGap(k *prep, sl Slot, filt bool, ng int, gi *[maxGrants]int16) bool {
	if filt && s.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && s.filtFee(k, sl) > 0 && s.filtUnits(k, sl) >= 2 {
		return true // the filter's fee: a pip, a hybrid pip or a reduced generic
	}
	c := s.cardAt(sl)
	if s.ownManaUsable(k, sl) && k.gen.flags[c]&(gfManaGate|gfManaRefl) == 0 {
		if pc := &k.cards[c]; pc.manaAb >= 0 && !pc.manaDyn && pc.manaN >= 2 &&
			s.manaAbCosts(k, sl, &k.t.Cards[c].Abilities[pc.manaAb]) {
			return true // the printed ability beside the grants (its own ask in the search)
		}
	}
	g := k.gen
	for j := 0; j < ng; j++ {
		gi2 := gi[j]
		if pc := s.grantView(g, gi2, sl); !pc.manaDyn && pc.manaN >= 2 &&
			s.manaAbCosts(k, sl, &k.t.Cards[g.grantOf[gi2].card].Abilities[g.grantOf[gi2].ab]) {
			return true // a granted multi-unit ability that costs mana
		}
	}
	return false
}

// manaAbCosts reports whether a's mana cost (after the activated-ability
// reductions that reach it, gorge manaAbilityCC) asks mana: a generic or
// coloured symbol, a hybrid one or {X} (potentialPrefixScript's
// cc.Cost.CMC() > 0 || cc.Cost.X > 0).
func (s *State) manaAbCosts(k *prep, sl Slot, a *kop.Ability) bool {
	m := a.Cost.Mana
	if k.gen.anyAbReduce {
		m = s.abReduced(k, a, sl, s.perms[sl].ctrl, m)
	}
	if m.X > 0 {
		return true
	}
	return m.Generic > 0 || m.Pips != [kop.NumManaKinds]uint8{} || m.Hyb != [4]kop.ColorMask{}
}

// feeOrderFallback reports whether gorge's fee-order run follows the plain run
// fs, which found no script: a plain cast of the printed cost on a board of
// filter gaps alone, one of them a fee gap, whose own cost sacrifices or
// returns nothing and whose gaps hold no hybrid fee on any mana ability of
// their faces (gapFaceHybIn; those are the phased order's own scope, gorge
// phasedScope). The plain run's budget must have run out: a completed search
// saw every state the reordered one reaches. It records the fee gaps on fs —
// only on success, so a declined fallback leaves the plain run's search as it
// was (every read is gated on fs.feeOrder, but the field is the fallback's to
// set).
func (s *State) feeOrderFallback(k *prep, p uint8, a Action, fs *filtSearch) bool {
	if fs.pay != nil || fs.other.any() || fs.nodes < potentialScriptNodes || a.Kind() != AkCast || a.Mode() != 0 {
		return false
	}
	feeGaps := s.filtFeeGaps(k, fs.gaps)
	if !feeGaps.any() || s.gapFaceHybIn(k, fs.gaps) {
		return false
	}
	o := Obj(a.Source())
	if o == NoObj || int(o) >= len(s.objs) {
		return false
	}
	ac := spellAddCost(k, s.objs[o].card)
	if ac.Flags&(kop.CostSac|kop.CostSacSelf|kop.CostReturn|kop.CostReturnSelf) != 0 {
		return false
	}
	fs.feeGaps = feeGaps
	return true
}
