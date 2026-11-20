package oracle

// The flag and comparison words of kgen's property table beyond the type and
// colour words of atomdiff_fw_base.go: the permanent state flags, the
// attachment and damage reads, the P/T comparisons between statistics, the
// counters_<op><n>_<kind> word and the source-relative choice words.
func init() {
	for _, w := range []string{
		"Card.StrictlySelf", "Card.StrictlyOther", "Card.tokenCreated", "Card.!token",
		"Card.AttachedBy", "Card.!EnchantedBy", "Card.!EquippedBy", "Card.!AttachedBy", "Card.!ThisTurnEntered",
		"Card.wasDealtDamageThisTurn", "Card.!wasDealtDamageThisTurn", "Card.DamagedBy", "Card.CrewedThisTurn", "Card.SaddledThisTurn", "Card.IsSaddled",
		"Card.ChosenType", "Card.ChosenColor", "Card.IsMonstrous", "Card.AdventureCard",
		"Creature.powerGTbasePower", "Creature.powerLTtoughness", "Creature.equalPT", "Creature.!equalPT",
		"Card.counters_GE1_P1P1", "Card.counters_EQ0_P1P1", "Card.counters_GE2_P1P1", "Card.counters_GE1_CHARGE",
		"Creature.YouCtrl+tapped", "Creature.OppCtrl+attacking", "Creature.nonArtifact+nonBlack", "Permanent.nonLand+YouCtrl",
		"Card.Other+YouCtrl", "Creature.token+untapped",
	} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
	}
}
