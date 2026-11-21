package kop

// RestrictedLifeShape reports whether mana ability a is a spend-restricted
// "{T}, Pay N life: Add <mana>" of one fixed colour or colourless, one or
// several mana, or of any colour one amount below or at two (Thieving
// Varmint's two mana of any one colour), with a spell restriction and no
// flag, rider, program or each-colour production (Gnarlroot Trapper's
// Elf-only {G}, Untaidake's legendary-only {C}{C}): the shape the kernel
// plans as a last-resort unit (dirac mana_restrictedcost.go). A colour
// choice among printed colours (Produced$ Combo) is a window gap the kernel
// does not price with a life cost.
func RestrictedLifeShape(a *Ability) bool {
	c := a.ManaColors & 0x3f
	return a.Kind == AbMana && a.ManaRestrict != 0 && a.ManaRestrictAb == 0 && a.Cost.Flags == CostTap|CostPayLife && a.Cost.N >= 1 &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.AFlags == 0 && !a.ManaEach && !a.ManaDeferred && len(a.Prog) == 0 &&
		a.ManaRider == ManaRiderNone && a.ManaN <= 2 && c != 0 && (c&(c-1) == 0 || c == ColAny) && a.ManaColors&^ColAny&^ColC == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield)
}

// RestrictedLifeManaAb reports whether ability j of cd is the card's only own
// mana ability and is a RestrictedLifeShape: a last-resort planner unit of
// gorge's census (pay.PaymentPlanShapeTierOf's cost.Life arm) for exactly the
// payments its restriction admits.
func RestrictedLifeManaAb(cd *CardDef, j int) bool {
	if !RestrictedLifeShape(&cd.Abilities[j]) {
		return false
	}
	for i := range cd.Abilities {
		if i != j && cd.Abilities[i].Kind == AbMana && cd.Abilities[i].AFlags&AfLatent == 0 {
			return false
		}
	}
	return true
}
