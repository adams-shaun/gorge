package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// (gorge, CR 116.2, 106.6) a special action's mana
// is its own payment class (gorge pay.PurposeSpecial), not a spell of the
// card. Plot is the kernel's special action that spends floating or
// tapped restricted mana. kgen lowers only Spell.<spec>, Activated.<spec>
// and CantCastNonArtifactSpells (the Powerstone, AfManaNonSpell): of
// those gorge's RestrictValidMatches admits only the last for a special
// action, which is exactly restrSrcAdmits for the pdOther class (every
// non-spell payment that is not an activated ability), so the plot is
// priced as pdOther and no class constant is added. Cold code, out of
// line.

// restrPoolNonSpell is the mask of p's restricted batches (State.rb) whose
// restriction admits a non-spell payment: a source of the table with that
// spend restriction carries AfManaNonSpell. A batch names its restriction by
// genPrep.restrF index, not the source.
//
//go:noinline
func (s *State) restrPoolNonSpell(k *prep, p uint8) uint8 {
	if k.gen == nil {
		return 0
	}
	var m uint8
	for i := range s.rb[SeatIx(p)] {
		b := &s.rb[SeatIx(p)][i]
		if b.kind == 0 {
			break
		}
		if b.units() != 0 && s.restrNonSpellKind(k, b.kind) {
			m |= 1 << i
		}
	}
	return m
}

// restrNonSpellKind: a source of the table with the spend restriction batch kind kind names admits a
// non-spell payment.
func (s *State) restrNonSpellKind(k *prep, kind uint8) bool {
	// the SpmActivated marker batch holds no AfManaNonSpell ability: refused here on purpose
	for i := range k.t.Cards {
		abs := k.t.Cards[i].Abilities
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbMana && k.gen.restrIsKind(kind, a) && a.AFlags&kop.AfManaNonSpell != 0 {
				return true
			}
		}
	}
	return false
}

// plotRestrFallback: the plot action of p, which av does not pay, still
// pays through p's restricted mana the restriction of which admits a
// non-spell payment, or the restricted sources that admit one (pdOther).
// A spell-only restriction (Villages, Giada, Kolvori, Courtyard's spell
// half, Helga's float) funds nothing here.
//
//go:noinline
func (s *State) plotRestrFallback(k *prep, p uint8, cost *kop.ManaCost, av *avail) bool {
	t := *av
	if m := s.restrPoolNonSpell(k, p); m != 0 {
		s.restrAvailAdd(p, m, &t)
		if t.affords(cost, pipMaskOf(cost), 0) {
			return true
		}
	}
	if !k.restrSrc {
		return false
	}
	// restrSrcNone precondition: t does not pay (tested above and by the caller).
	return s.restrSrcAffords(k, p, payDesc{class: pdOther, src: NoSlot}, cost, &t, NoSlot, 0)
}

// payPlotRestr pays the plot action while restricted mana floats: a
// non-spell-admitting restriction's mana is spent before the ordinary mana
// of its colour (gorge's restricted-batch carve for a special descriptor);
// a spell-only one stays beside the pool.
//
//go:noinline
func (s *State) payPlotRestr(k *prep, p uint8, cost *kop.ManaCost, o Obj) {
	if m := s.restrPoolNonSpell(k, p); m != 0 {
		s.payRestrSpend(k, p, m, cost, 0, o, false)
		return
	}
	s.payFromPool(p, cost, 0)
}

// nonSpellRestrAffords: p pays cost, a payment that is no spell's (a ward's {N}),
// counting the restricted mana its restriction admits there (CR 106.6): the
// floating batches and the restricted sources, Hydraulic Helper's mana "can't be
// spent to cast a nonartifact spell" paying a ward.
//
//go:noinline
func (s *State) nonSpellRestrAffords(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.restrSrc && !s.rbAny(p&1) {
		return false
	}
	var av avail
	s.availWindowFor(k, p, &av)
	return s.plotRestrFallback(k, p, cost, &av)
}

// restrPoolSpecial is the mask of p's restricted mana batches (State.rb) whose
// RestrictValid$ Static.* clause admits the special action sp on the paid
// object at sl (CR 106.6, 116.2): turning that permanent face up or unlocking
// that Room door. A spell- or activated-ability-only batch admits none.
//
//go:noinline
func (s *State) restrPoolSpecial(k *prep, p uint8, sp kop.SpecialMask, sl Slot) uint8 {
	if k.gen == nil || sp == 0 {
		return 0
	}
	var m uint8
	for i := range s.rb[SeatIx(p)] {
		b := &s.rb[SeatIx(p)][i]
		if b.kind == 0 {
			break
		}
		if b.units() != 0 && s.restrSpecialFilter(k, p, NoSlot, k.gen.restrFOf(b.kind), sp, sl) {
			m |= 1 << i
		}
	}
	return m
}

// restrSpecialFilter reports whether the restriction filter f has a Static.*
// alternative admitting sp on the object at target. src is the mana source's
// slot when known (a source-relative predicate), NoSlot for a floating batch.
func (s *State) restrSpecialFilter(k *prep, p uint8, src Slot, f kop.FilterID, sp kop.SpecialMask, target Slot) bool {
	for ; f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Special&sp == 0 {
			continue // sp never holds kop.SpmActivated: refused here on purpose
		}
		fl.Special = 0
		fl.Zone = kop.ZoneNone
		fl.Flags2 &^= kop.F2ChosenType
		if fl == (kop.Filter{}) {
			return true // bare Static (or an SA-property-only tail): every object
		}
		if target != NoSlot && s.matchPermOne(k, &fl, target, p, src, layerFull) {
			return true
		}
	}
	return false
}

// specialAffords reports whether p's floating pool, with the restricted mana a
// special action's restriction admits, pays cost at the announced X = x: gorge
// prices a special action against AvailableFor's pool alone
// (pay.CostPayableSpecial), never tapping a source, so the offer and the
// payment read the same mana. x is 0 unless the action announces an X (a
// keyword turn-up's Disguise {X}).
//
//go:noinline
func (s *State) specialAffords(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, x int) bool {
	m := s.restrPoolSpecial(k, p, sp, sl)
	if m == 0 && x == 0 {
		return s.poolPays(p, cost)
	}
	var a avail
	pl := &s.pl[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<c, pl.pool[c])
		}
	}
	if m != 0 {
		s.restrAvailAdd(p, m, &a)
	}
	cx := cost.AtX(x)
	return a.affords(&cx, pipMaskOf(&cx), 0)
}

// paySpecial spends a special action's mana: the restricted batches its
// Static.* restriction admits first (gorge's emitRestrictedManaSpend carves
// them out), then the ordinary pool; what the payment leaves floats back under
// its restriction.
//
//go:noinline
func (s *State) paySpecial(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, x int) {
	if m := s.restrPoolSpecial(k, p, sp, sl); m != 0 {
		s.payRestrSpend(k, p, m, cost, x, NoObj, false)
		return
	}
	s.payFromPool(p, cost, x)
}
