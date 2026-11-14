package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Produced$ Combo Any mana ability (AfManaChoice over every colour, not
// reflected) tapped for nothing more: Axebane Guardian, Sage of the Maze,
// Key to the Archive, Prismatic Geoscope. gorge's payment planner has no
// witness for it (rules pay PaymentPlanUnitAlternatives keeps Combo Any
// manual: PaymentPlanChoiceColours fails closed on its token), so the source
// is no plan unit and adds no colour to the planner's ranking; the census
// lists it as a gap. A plan the other sources make is gorge's, so the kernel
// plans without these sources first (planComboTiered) and counts them only
// when no such plan exists.

// DebugComboAnyPlanned makes a Combo Any source an ordinary plan unit again
// (the model gorge's planner does not follow), mutation testing only: the
// lockstep must catch it (TestComboAnySourceOffThePlanMutation).
var DebugComboAnyPlanned bool

// comboAnyMana: a is a {T} Combo Any mana ability.
func comboAnyMana(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.AFlags&kop.AfManaChoice != 0 && a.AFlags&kop.AfManaReflect == 0 &&
		a.ManaColors == kop.ColAny && !a.ManaIdentity && a.ManaRestrict == 0 && a.Cost.Flags == kop.CostTap &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// comboAnySrc reports whether card c's printed mana ability is one.
func comboAnySrc(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	for j := range cd.Abilities {
		if comboAnyMana(&cd.Abilities[j]) {
			return true
		}
	}
	return false
}

// tableComboAny reports whether t has a Combo Any source (the gate).
func tableComboAny(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if comboAnyMana(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// planComboTiered is planTiered with the Combo Any sources left out of the
// first pass: only a cost the other sources cannot plan counts them.
func (s *State) planComboTiered(k *prep, p uint8, pk *planner, exclude Obj) {
	saved := *pk
	pk.try = true
	s.planTiered0(k, p, pk, exclude)
	if pk.found {
		pk.try = saved.try
		return
	}
	*pk = saved
	pk.comboIn = true
	s.planTiered0(k, p, pk, exclude)
}
