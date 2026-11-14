package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A {T} mana ability adding an evaluated amount of mana in any combination of
// colours (Axebane Guardian's "Add X mana in any combination of colors, where
// X is the number of creatures with defender you control": Produced$ Combo
// Any, Amount$ X). gorge's cast planner cannot price it (a Combo Any has no
// colour list, pay.PaymentPlanChoiceColours fails closed), so it is no unit of
// the planner's census: a cast only it pays is no planned cast, and the
// autopay surface does not offer it from the sources (CR 605.1a, 601.2g).

// comboDynAb reports whether a is such an ability.
func comboDynAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.AFlags&kop.AfManaChoice != 0 && a.AFlags&kop.AfManaDynamic != 0 &&
		a.ManaColors&kop.ColAny == kop.ColAny
}

// tableComboDyn reports whether t has such an ability.
func tableComboDyn(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if comboDynAb(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// comboDynAt reports whether the mana ability pc makes of the source at sl is
// such an ability.
func (s *State) comboDynAt(k *prep, sl Slot, pc *pcard) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	return pc.manaAb >= 0 && int(pc.manaAb) < len(cd.Abilities) && comboDynAb(&cd.Abilities[pc.manaAb])
}

// comboDynLive reports whether p controls an untapped source whose mana
// ability is such an ability.
func (s *State) comboDynLive(k *prep, p uint8) bool {
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped == 0 && s.comboDynAt(k, sl, s.manaPCGen(k, sl)) {
			return true
		}
	}
	return false
}
