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
	for i := range s.rb[p&1] {
		b := &s.rb[p&1][i]
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
