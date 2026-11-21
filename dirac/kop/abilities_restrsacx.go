package kop

// SacXRestrManaAb reports whether mana ability a is a spend-restricted
// "{T}, Sacrifice another <filter>: Add <one fixed colour>" (Master of Dark
// Rites' "{T}, Sacrifice another creature: Add {B}{B}{B}. Spend this mana only
// to cast Vampire, Cleric, and/or Demon spells"): the SacTapFixedManaAb shape
// beside a spell restriction, up to three mana of one colour or of {C}.
// gorge's census defers it like the unrestricted one (pay.PaymentPlanShapeTierOf:
// a Sac part that is not the source), and the kernel's tap-another search
// activates it (dirac mana_tapanother.go), its mana floating restricted
// (CR 106.6).
func SacXRestrManaAb(a *Ability) bool {
	c := a.ManaColors
	return a.Kind == AbMana && a.ManaRestrict != 0 && a.ManaRestrictAb == 0 && !a.ManaEach && !a.ManaDeferred &&
		a.Cost.Flags == CostTap|CostSac && a.Cost.N == 1 && a.Cost.Filter != 0 && !a.Cost.ExileBf && !a.Cost.DiscardHand &&
		a.Cost.Mana == (ManaCost{}) && a.AFlags == 0 && len(a.Prog) == 0 && a.ManaRider == ManaRiderNone &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield) &&
		a.ManaN >= 1 && a.ManaN <= 3 && c != 0 && c&(c-1) == 0 && c&^ColAny == 0
}
