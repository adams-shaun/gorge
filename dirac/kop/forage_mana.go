package kop

// ForageManaCost reports whether cost c is exactly {T} plus a forage part (CR
// 701.61a): Cost.Filter holds the Food the forage may sacrifice, and nothing else
// (no mana, no other counted part) rides the cost.
func ForageManaCost(c *Cost) bool {
	return c.Flags == CostTap|CostForage && c.Filter != 0 && c.N == 0 && c.Mana == (ManaCost{}) && c.Host == 0 && c.Ctr == 0 &&
		!c.PutLibSelf && !c.ExileHand && !c.ExileBf && !c.ExileGSelf && !c.Exert && !c.DiscardHand && !c.DiscardRandom
}

// ForageComboAnyAb reports whether mana ability a is "{T}, Forage: Add two mana in
// any combination of colors" (Thornvault Forager): the unrestricted Combo Any
// production (a dynamic amount or at least two mana of any colour, AfManaChoice
// with ColAny) paid with {T} plus a forage and nothing else, no rider, gate or
// activation zone. gorge pays the forage in its mana-activation walk (pay
// manacostwalk.go, AskManaForage: forage_exile first, then one forage_food per Food;
// asked only with two or more options), its planner prices neither the forage nor
// the combination, so the source is a Combo Any census gap whose prefix-search
// activation answers the forage first (kernel mana_forage.go).
func ForageComboAnyAb(a *Ability) bool {
	return a.Kind == AbMana && ForageManaCost(&a.Cost) &&
		a.AFlags&(AfManaChoice|AfLatent|AfManaEachColor|AfManaChosen|AfManaReflect) == AfManaChoice &&
		a.ManaColors == ColAny && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && !a.ManaDeferred && !a.ManaEach &&
		a.ManaRider == ManaRiderNone && a.SplitGate == 0 && a.SplitCols == 0 && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && (a.ActZone == ZoneNone || a.ActZone == ZoneBattlefield) &&
		(a.AFlags&AfManaDynamic != 0 || a.ManaN >= 2)
}
