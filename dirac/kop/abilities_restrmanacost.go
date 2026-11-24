package kop

// RestrictedManaCostShape reports whether mana ability a is a spend-restricted
// one whose cost is literal mana plus {T} (Castle Garenbrig's "{2}{G}{G}, {T}:
// Add six {G}", creature spells and creature abilities only): gorge's
// cast-window probe prices it like an unrestricted one, the fee paid from the
// floating mana before the production joins (engine/rules cast_window.go
// castWindowGenericCostShape, CR 605.3b, 601.2g). One fixed colour or {C} of a
// literal amount, no {X}, hybrid, two-brid, {C} or snow part in the fee or the
// production, no choice, each-colour production, rider, program, gate or
// flag; the spell and the activated-ability restriction are both allowed. The
// kernel plans it as a restricted pair/sole unit (dirac
// mana_restrictedsource.go) whose fee is charged from the payment's floating
// mana.
func RestrictedManaCostShape(a *Ability) bool {
	if a.Kind != AbMana || a.AFlags&^(AfManaNonSpell|AfManaChosen) != 0 || a.ManaEach ||
		a.ManaColors == 0 || a.ManaRider != ManaRiderNone || len(a.Prog) != 0 || a.Gate.Ctr != 0 || a.Trig.Cond != 0 ||
		a.ActZone != ZoneNone && a.ActZone != ZoneBattlefield {
		return false
	}
	c := a.ManaColors
	if c != 0 && c&(c-1) != 0 {
		return false
	}
	m := &a.Cost.Mana
	if m.Pips[5] != 0 {
		return false // a {C} pip in the fee has no kernel model
	}
	return (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) &&
		a.Cost.Flags == CostTap && m.MV() != 0 && m.X == 0 && m.XRed == 0 && m.XPip == 0 &&
		m.Snow == 0 && m.Tb == 0 && m.Hyb == [4]ColorMask{} &&
		restrFeeOneColour(m)
}

// restrFeeOneColour: the fee's coloured pips name at most one colour (any
// count of it): the fee a kernel fee unit carries (pcard.manaFeeN) is one pip
// mask and one count.
func restrFeeOneColour(m *ManaCost) bool {
	n := 0
	for i := 0; i < 5; i++ {
		if m.Pips[i] != 0 {
			n++
		}
	}
	return n <= 1
}
