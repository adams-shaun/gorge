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

// restrPoolNonSpell reports whether p's restricted mana (poolR, with its
// other colours) came from a restriction that admits a non-spell payment:
// a source of the table with that spend restriction carries AfManaNonSpell.
// The byte names the restriction by genPrep.restrF index, not the source.
//
//go:noinline
func (s *State) restrPoolNonSpell(k *prep, p uint8) bool {
	r := s.pl[p&1].poolR
	if r&poolRCnt == 0 || k.gen == nil {
		return false
	}
	f := k.gen.restrF[0]
	if r&poolRKind != 0 {
		f = k.gen.restrF[1]
	}
	for i := range k.t.Cards {
		abs := k.t.Cards[i].Abilities
		for j := range abs {
			if a := &abs[j]; a.Kind == kop.AbMana && a.ManaRestrict == f && a.AFlags&kop.AfManaNonSpell != 0 {
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
	if s.pl[p].poolR != 0 && s.restrPoolNonSpell(k, p) {
		s.restrAvailAdd(p, &t)
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
	if s.restrPoolNonSpell(k, p) {
		s.payRestrSpend(k, p, cost, 0, o, false)
		return
	}
	s.payFromPool(p, cost, 0)
}
