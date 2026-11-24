package kop

// RestrictedManaCostShape reports whether mana ability a is a spend-restricted
// one whose cost is literal mana plus {T} (Castle Garenbrig's "{2}{G}{G}, {T}:
// Add six {G}", creature spells and creature abilities only): gorge's
// cast-window probe prices it like an unrestricted one, the fee paid from the
// floating mana before the production joins (engine/rules cast_window.go
// castWindowGenericCostShape, CR 605.3b, 601.2g). The production is one the
// kernel's restricted unit carries: one fixed colour or {C}, or N mana in any
// combination of colours (Combo Any); one mana of each of a fixed set of
// colours (Cormela's {U}{B}{R}) is the unit's first colour beside a fixed rest
// (prepEachMana); two mana of different colours (Interplanar Beacon's Combo AnyDifferent,
// RestrDistinctPair) is two any-colour units the availability model couples
// (dirac avail.nDist); one mana of a single chosen colour is no unit. No {X}, hybrid, two-brid, {C} or
// snow part in the fee, no rider, program, gate or evaluated amount; the spell
// and the activated-ability restriction are both allowed. The kernel plans it
// as a restricted pair/sole unit (dirac mana_restrictedsource.go) whose fee is
// charged from the payment's floating mana.
func RestrictedManaCostShape(a *Ability) bool {
	if a.Kind != AbMana || a.AFlags&^(AfManaNonSpell|AfManaChosen|AfManaChoice) != 0 ||
		a.ManaColors == 0 && !restrDistinctPair(a) || a.ManaRider != ManaRiderNone || len(a.Prog) != 0 || a.Gate.Ctr != 0 || a.Trig.Cond != 0 ||
		a.ActZone != ZoneNone && a.ActZone != ZoneBattlefield {
		return false
	}
	if !restrFeeProduction(a) {
		return false
	}
	m := &a.Cost.Mana
	if m.Pips[5] != 0 {
		return false // a {C} pip in the fee has no kernel model
	}
	if restrDistinctPair(a) && m.Pips != [6]uint8{} {
		return false // a distinct pair's fee is generic mana only (the kernel pays it from the pool or another pair source's {C})
	}
	return (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) &&
		a.Cost.Flags == CostTap && m.MV() != 0 && m.X == 0 && m.XRed == 0 && m.XPip == 0 &&
		m.Snow == 0 && m.Tb == 0 && m.Hyb == [4]ColorMask{} &&
		restrFeeOneColour(m)
}

// restrFeeProduction reports whether a's production is a shape the fee unit
// carries: one fixed colour or {C} of a literal amount, or N mana in any
// combination of colours (Produced$ Combo Any), or one mana of each of a fixed set
// of colours (ManaEach), or one mana of any colour (Produced$ Any of amount
// one), or two mana of different colours (RestrDistinctPair). Anything else
// (Produced$ Any of more, a Combo AnyDifferent of another amount) is no unit.
func restrFeeProduction(a *Ability) bool {
	n := int(a.ManaN)
	if n == 0 {
		n = 1
	}
	if n > maxRestrictedFeeAmount {
		return false
	}
	cols := a.ManaColors & (ColAny | ColC)
	switch {
	case restrDistinctPair(a):
		return true
	case a.ManaEach:
		// one mana of each of a fixed set of colours (Cormela's {U}{B}{R}):
		// the unit's first colour beside a fixed rest (prepEachMana)
		return a.AFlags&AfManaChoice == 0 && cols == a.ManaColors&0x3f && cols&ColC == 0 && cols&(cols-1) != 0
	case cols == 0:
		return false
	case a.AFlags&AfManaChoice != 0:
		// Combo Any: each unit is any of the five, the availability model
		// carries it as an any-colour group
		return cols == ColAny
	case cols == ColAny:
		// one mana of any colour (Hydro-Channeler's Produced$ Any beside its
		// free restricted {U}): one any-colour unit, no allocation
		return n == 1
	default:
		// one fixed colour or {C}
		return cols&(cols-1) == 0
	}
}

// maxRestrictedFeeAmount bounds the literal amount of a restricted mana-fee
// unit: kgen's maxRestrictedFeeAmount (lower_multimana.go), the largest
// production a fee source priced as a planner unit carries (Castle
// Garenbrig's six).
const maxRestrictedFeeAmount = 7

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

// RestrDistinctPair reports whether a's production is two mana of different
// colours (Produced$ Combo AnyDifferent, Amount$ 2: Interplanar Beacon): the
// Combo$ parse sets AfManaChoice and names no colour, so ManaColors is empty.
// The kernel carries it as two any-colour units the availability model couples
// (dirac avail.nDist).
func RestrDistinctPair(a *Ability) bool { return restrDistinctPair(a) }

func restrDistinctPair(a *Ability) bool {
	return a.Kind == AbMana && a.ManaColors == 0 && a.ManaN == 2 && a.AFlags&(AfManaChoice|AfManaEachColor|AfManaChosen|AfManaReflect|AfManaDynamic|AfLatent) == AfManaChoice &&
		!a.ManaEach
}
