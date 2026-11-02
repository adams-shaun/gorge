package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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

// ctrManaAb reports whether mana ability a costs only counters put on its
// source, once each turn, with a fixed production.
func ctrManaAb(a *kop.Ability) bool { return kop.CtrManaAb(a) }

// scriptCtr: p's counter-cost sources that can be activated now (no {T}:
// summoning sickness does not matter, CR 302.6), a slot mask.
//
//go:noinline
func (s *State) scriptCtr(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana || DebugNoCtrMana {
		return m
	}
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		sl := b.first()
		pc := &k.cards[s.cardAt(sl)]
		if !pc.ctrMana || s.perms[sl].actUsed&(1<<uint(pc.manaAb)) != 0 || s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
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
		pc := &k.cards[s.cardAt(b.first())]
		av.addUnits(uint8(pc.manaCol), pc.manaN)
	}
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
	return !decided || uint8(k.cards[s.cardAt(o)].manaCol)&want != 0
}

// ctrGap activates counter-cost source sl for p: its counters, its use this
// turn, its mana into the pool (gorge's activation: the cost's
// CounterChange, then the mana).
//
//go:noinline
func (s *State) ctrGap(k *prep, p uint8, sl Slot) {
	pc := &k.cards[s.cardAt(sl)]
	a := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]
	pm := &s.perms[sl]
	n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), p, ctrPutCost) // a cost: only the non-EffectOnly$ replacements
	if n > 0 && k.anyCtrAdded {
		s.ctrNoteBy(k, sl, int(a.Cost.Ctr), p) // a cost: gorge pay_engine publishes the payer
	}
	pm.actUsed |= 1 << uint(pc.manaAb)
	s.touchSl(sl)
	for c := 0; c < kop.NumManaKinds; c++ {
		if uint8(pc.manaCol)&(1<<c) != 0 {
			s.pl[p&1].pool[c] += pc.manaN
		}
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
