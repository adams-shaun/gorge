package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Firemind Vessel's "{T}: Add two mana of different colors" (Produced$ Combo
// AnyDifferent). gorge's potential walk (rules potential.go addPotentialMana)
// prices the production unbounded in every colour: producedOpen is true for the
// AnyDifferent word, so an untapped one makes any trigger's own mana cost look
// payable (Insidious Mist's "you may pay {2}{B}" is offered beside the decline
// with every land tapped). The production itself resolves to no mana (the
// effect's rune gate, as for Component Pouch, kop.CtrDeadManaAb), so the kernel
// models the source as a dead one that adds no units; this reads the open
// over-bound the offer gate needs and nothing else.

// anyDifferentTapAb reports whether a is the plain {T} AnyDifferent mana
// ability: a mana-free cost of {T}, the choice flag the Combo$ parse sets and no
// colour, restriction, rider or program.
func anyDifferentTapAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == kop.AfManaChoice && a.ManaColors == 0 && a.ManaN > 0 && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		!a.ManaEach && !a.ManaDeferred && a.ManaRider == kop.ManaRiderNone && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && a.SplitGate == kop.SplitGateNone &&
		(a.ActZone == kop.ZoneNone || a.ActZone == kop.ZoneBattlefield)
}

// anyDifferentAt reports whether the permanent at sl is an untapped, usable
// source with the plain {T} AnyDifferent mana ability. The prep gives the
// ability no pcard slot (its colour mask is empty, gen.go), so the card's
// abilities are read directly.
func (s *State) anyDifferentAt(k *prep, sl Slot) bool {
	pp := &s.perms[sl]
	if pp.flags&pfTapped != 0 || pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) || s.activationSick(k, sl) {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		if anyDifferentTapAb(&cd.Abilities[j]) {
			return true
		}
	}
	return false
}

// anyDifferentGaps is p's usable AnyDifferent sources as a slot mask: the dead
// census gaps gorge's prefix search activates in battlefield order beside the
// gaps that fund a play (scriptSearch.visit), each tapped for no mana.
//
//go:noinline
func (s *State) anyDifferentGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if k.gen == nil {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		if sl := b.first(); s.anyDifferentAt(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// anyDifferentOpen reports whether gorge's potential pool is unbounded in every
// colour for p: an untapped, usable AnyDifferent source.
//
//go:noinline
func (s *State) anyDifferentOpen(k *prep, p uint8) bool {
	if k.gen == nil {
		return false
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		if s.anyDifferentAt(k, m.first()) {
			return true
		}
	}
	return false
}

// windowAnyDifferentTap is the trigger-cost window's activation of an
// AnyDifferent source o that windowPick listed: the planner lists it in its
// battlefield place like any other source, and the production resolves to no
// mana ("unhandled Produced$ AnyDifferent"), so the window taps it and floats
// nothing (as ctrDeadGap does for Component Pouch).
//
//go:noinline
func (s *State) windowAnyDifferentTap(k *prep, o Slot) {
	s.perms[o].flags |= pfTapped
	s.manaTapped(k, o)
	s.touchSl(o)
	s.tapExtraStall(k, o)
}

// windowGapListed is windowPick's gap test: a Tender, restricted or Combo Any
// source the window lists (windowGap), or an AnyDifferent source, which the
// planner lists in its battlefield place in every window (a trigger cost's, a
// Mystic Remora's pay-{4} ask), not only the potential one.
func (s *State) windowGapListed(k *prep, p uint8, o Slot) bool {
	return k.gen != nil && (k.gen.anyWindowGap && s.windowGap(k, p, o) || s.anyDifferentAt(k, o))
}

// availPotentialPay is availPotentialFor for a trigger cost's payment: an
// untapped AnyDifferent source makes gorge's potential pool unbounded in every
// colour (addPotentialMana prices Produced$ AnyDifferent open), though the
// kernel's walk reads its empty colour mask as nothing.
func (s *State) availPotentialPay(k *prep, p uint8, a *avail, d *payDesc) {
	s.availPotentialFor(k, p, a, d)
	if s.anyDifferentOpen(k, p) {
		a.total, a.n = 0, 0
		for c := 0; c < kop.NumManaKinds; c++ {
			a.addUnits(1<<c, uint8(min(potentialOpenUnits, 255)))
		}
	}
}
