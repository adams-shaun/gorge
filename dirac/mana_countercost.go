package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Counter-cost mana (Wall of Roots: "Put a -0/-1 counter on Wall of
// Roots: Add {G}. Activate only once each turn."). gorge's planner census
// cannot execute an AddCounter cost (pay.PlanLastResortCostOK), so the
// source is a census gap with a fixed relaxed alternative (potential_plan.go
// paymentPlanRelaxProof: "Wall of Roots' counter"): a play the lands cannot
// pay but the relaxed census can is searched for a prefix that activates
// the gap first (potentialPrefixScript), the sources then paying the rest.
// The kernel models it as one more script gap kind beside the each-colour,
// restricted and sacrifice-only ones (script_gen.go): its mana in the gaps
// walk (availCtr), and at the play the activations in battlefield order
// until the plain walk lists it (ctrGap). Each activation puts a -0/-1
// counter (kop.CtrM0M1, read by ptGen) and spends the once-each-turn use
// (perm.actUsed). Everything is behind prep.ctrMana.
//
// The all-at-once removal shape (kop.CtrEachManaAb: Ramos, Dragon Engine's
// "Remove five +1/+1 counters from Ramos: Add {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}.
// Activate only once each turn.") is the same gap: gorge relaxes it to one
// free alternative holding the whole production. Its pcard keeps the whole
// colour mask (no prepEachMana split: it is no tap source), the gaps walk
// adds every colour (not one of them), the activation removes the counters
// as the activation path's CostRemoveCtr does, and the gap is offered only
// while the source holds them (gorge manaAbilityPayablePool).

// ctrManaAb reports whether mana ability a costs only counters put on its
// source, once each turn, with a fixed production.
func ctrManaAb(a *kop.Ability) bool { return kop.CtrManaAb(a) }

// ctrAbOf is card c's counter-cost mana ability (prep.ctrAb): its first mana
// ability (Wall of Roots, Ramos), or a later {T} one beside a plain {T} first
// (Weather Maker), nil none.
func (k *prep) ctrAbOf(c CardID) *kop.Ability {
	if int(c) >= len(k.ctrAb) || k.ctrAb[c] < 0 {
		return nil
	}
	return &k.t.Cards[c].Abilities[k.ctrAb[c]]
}

// ctrSecond: card c's counter-cost ability is a later one beside a plain {T}
// first (Weather Maker), so activating the source asks which ability. A
// storage battery whose first ability is the counter one (Bottomless Vault)
// has no plain first ability to answer with (pcard.manaAb is -1: a dynamic
// amount is no planner unit), so it is not a second.
func (k *prep) ctrSecond(c CardID) bool {
	return int(c) < len(k.ctrAb) && k.ctrAb[c] >= 0 && k.cards[c].manaAb >= 0 && k.ctrAb[c] != k.cards[c].manaAb
}

// scriptCtr: p's counter-cost sources that can be activated now (a Wall of
// Roots has no {T}: summoning sickness does not matter, CR 302.6; a source
// whose cost taps it must be untapped and, a creature, not sick), a slot mask.
//
//go:noinline
func (s *State) scriptCtr(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana || DebugNoCtrMana {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		a := k.ctrAbOf(c)
		if a == nil || s.perms[sl].actUsed&(1<<uint(k.ctrAb[c])) != 0 || s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if a.Cost.Flags&kop.CostRemoveCtr != 0 && s.perms[sl].ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
			continue // too few counters to remove (legal.go actCostOK's CostRemoveCtr)
		}
		if kop.StoreManaAb(a) {
			// A storage battery: the announced X is the counters it holds, so
			// with none there is no activation. Its {T} is read here, not
			// through canTapForManaPC (whose plain unit prices one mana, not X).
			if s.perms[sl].ctrN(s, int(a.Cost.Ctr)) <= 0 {
				continue
			}
			if a.Cost.Flags&kop.CostTap != 0 && (s.perms[sl].flags&pfTapped != 0 || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.activationSick(k, sl)) {
				continue
			}
		} else if a.Cost.Flags&kop.CostTap != 0 && !s.canTapForMana(k, sl) {
			continue // its {T}: the source is untapped and not summoning sick (the second ability beside a plain {T} first)
		}
		m = m.with(sl)
	}
	return m
}

// availCtr adds p's counter-cost sources' mana to the gaps walk.
//
//go:noinline
func (s *State) availCtr(k *prep, p uint8, av *avail) {
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(s.cardAt(sl))
		if kop.StoreManaAb(a) {
			n := s.perms[sl].ctrN(s, int(a.Cost.Ctr))
			if n > 255 {
				s.capFull(StallDynMana)
				n = 255
			}
			if n > 0 {
				av.addUnits(uint8(a.ManaColors)&0x3f, uint8(n))
			}
			continue
		}
		addCtrUnits(av, a)
	}
}

// addCtrUnits adds counter-cost ability a's mana to av: every colour at once
// for the all-at-once shape (Ramos), else one unit of its fixed colour.
func addCtrUnits(av *avail, a *kop.Ability) {
	cols, n := tapXUnit(a)
	if a.ManaEach {
		for c := 0; c < kop.NumManaKinds; c++ {
			if cols&(1<<c) != 0 {
				av.addUnits(1<<c, n) // every colour at once, not a choice of one
			}
		}
		return
	}
	av.addUnits(cols, n)
}

// windowCtrOffer: counter-cost source o is one gorge's ward or unless window
// lists and can take now (it asks no colour), and its colour adds something the
// clamp keeps.
//
//go:noinline
func (s *State) windowCtrOffer(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if !k.ctrMana || !s.scriptCtr(k, p).has(o) {
		return false
	}
	return !decided || uint8(k.ctrAbOf(s.cardAt(o)).ManaColors)&want != 0
}

// ctrGap activates counter-cost source sl for p: its counters, its use this
// turn, its mana into the pool (gorge's activation: the cost's
// CounterChange, then the mana).
//
//go:noinline
func (s *State) ctrGap(k *prep, p uint8, sl Slot) {
	c := s.cardAt(sl)
	a := k.ctrAbOf(c)
	if kop.StoreManaAb(a) {
		s.storeGap(k, p, sl, a) // the storage battery's announced X (mana_countercost.go)
		return
	}
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		pm.seeCtr(s) // the activation path's CostRemoveCtr (mana_window.go)
		pm.subCtrN(s, int(a.Cost.Ctr), int(a.Cost.N))
	} else {
		n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), p, ctrPutCost) // a cost: only the non-EffectOnly$ replacements
		if n > 0 && k.anyCtrAdded {
			s.ctrNoteBy(k, sl, int(a.Cost.Ctr), p) // a cost: gorge pay_engine publishes the payer
		}
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 {
		pm.actUsed |= 1 << uint(k.ctrAb[c])
	}
	taps := a.Cost.Flags&kop.CostTap != 0
	if taps {
		pm.flags |= pfTapped // the second ability's {T} (Weather Maker)
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	cols, n := tapXUnit(a)
	for col := 0; col < kop.NumManaKinds; col++ {
		if cols&(1<<col) != 0 {
			s.poolAdd(p&1, col, int(n))
			if taps {
				s.artTag(k, p, sl, col)
			}
		}
	}
	if taps {
		s.tapExtraStall(k, sl)
	}
}

// sbaCtrTough is CR 704.5f for a creature whose toughness only -0/-1
// counters lowered: the SBA's no-damage shortcut reads the printed
// toughness, so the counters are read here, once per pass.
//
//go:noinline
func (s *State) sbaCtrTough(k *prep, die, gy *slotMask) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if die.has(sl) || s.perms[sl].ctrN(s, int(kop.CtrM0M1)) == 0 || !s.isCreature(k, sl) {
			continue
		}
		if _, t := s.pt(k, sl); t <= 0 {
			*die = die.with(sl)
			*gy = gy.with(sl)
		}
	}
}

// DebugNoCtrMana hides the counter-cost gaps (the wallroots mutation test
// proves the pool's plays need them).
var DebugNoCtrMana bool

// storeGap activates a single-colour storage-battery mana ability (Mage-Ring
// Network and the single-colour storage lands): its {T} if any marked, every
// counter of the kind removed, and that many mana of the ability's one
// colour. A generic fee, a colour combination or an any-colour production is
// not modelled (kgen refuses those shapes with their reasons). CR 107.3,
// 118.3; gorge's non-interactive ManaCostSubCounterStage takes X at the bound
// (all the counters).
func (s *State) storeGap(k *prep, p uint8, sl Slot, a *kop.Ability) {
	pm := &s.perms[sl]
	n := pm.ctrN(s, int(a.Cost.Ctr))
	if n <= 0 {
		return
	}
	if n > 255 {
		s.capFull(StallDynMana)
		n = 255
	}
	taps := a.Cost.Flags&kop.CostTap != 0
	if taps {
		if pm.flags&pfTapped != 0 {
			return
		}
		pm.flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	pm.seeCtr(s)
	pm.subCtrN(s, int(a.Cost.Ctr), n)
	col := bits.TrailingZeros8(uint8(a.ManaColors) & 0x3f)
	s.poolAdd(p&1, col, n)
	if taps {
		s.artTag(k, p, sl, col)
		s.tapExtraStall(k, sl)
	}
}
