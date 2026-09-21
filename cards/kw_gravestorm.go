// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwGravestorm(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.84: Storm's shape with a different amount -- one copy per
	// permanent put into a graveyard from the battlefield this turn
	// (Forge's CardFactoryUtil expansion). The count head resolves
	// through effects.countEntered's zone-aware spec match.
	f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Gravestorm",
		"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnEntered_Graveyard_from_Battlefield_Permanent | MayChooseTarget$ True", has)
}

func init() { registerKeyword(kwGravestorm, "Gravestorm") }
