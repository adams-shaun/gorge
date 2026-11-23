package kop

// RestrictedSacSelfShape reports whether mana ability a is a spend-restricted
// "{T}, Sacrifice <this>: Add <mana>" of any one colour or of one fixed
// colour or colourless, one to three mana, with a spell restriction and no
// flag, rider, program or each-colour production (Jeweled Lotus's three mana
// of any one colour, spent only to cast your commander): the shape the kernel
// plans as a last-resort sacrificing unit (dirac mana_restrictedcost.go), the
// restricted twin of a Treasure's {T}, Sacrifice ability (CR 106.6, 605.1a).
func RestrictedSacSelfShape(a *Ability) bool {
	c := a.ManaColors & 0x3f
	return a.Kind == AbMana && a.ManaRestrict != 0 && a.ManaRestrictAb == 0 && a.Cost.Flags == CostTap|CostSacSelf &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == 0 && !a.ManaEach && !a.ManaDeferred && len(a.Prog) == 0 &&
		a.ManaRider == ManaRiderNone && a.ManaN <= 3 && c != 0 && (c&(c-1) == 0 || c == ColAny) && a.ManaColors&^ColAny&^ColC == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield)
}
