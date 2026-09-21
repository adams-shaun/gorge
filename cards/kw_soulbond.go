// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwSoulbond(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.103 has two independently-triggering cases: this creature
	// enters, and another unpaired creature its controller controls
	// enters. The two synthetic KeywordLine suffixes retain idempotency
	// for both expansions while Keyword remains the printed keyword.
	//
	// The second case's partner choice must be restricted to the
	// SPECIFIC creature that triggered it (CR 702.103a: "you may pair
	// this creature with that creature"), not any unpaired creature
	// the controller happens to have -- a bystander unpaired creature
	// must never be offered just because a third, unrelated creature
	// entered. RestrictToRemembered$ True tells effPair (Ctx.Remembered
	// already carries the triggering entrant, via triggerRemembered) to
	// narrow its candidate scan to that one object; the #self trigger
	// omits it and keeps the broad "any unpaired creature I control"
	// scan CR 702.103a's other half calls for.
	f.addKeywordTrigger(head, k+"#self", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.Self | TriggerDescription$ Soulbond",
		"DB$ Pair", has)
	f.addKeywordTrigger(head, k+"#other", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | TriggerDescription$ Soulbond",
		"DB$ Pair | RestrictToRemembered$ True", has)
}

func init() { registerKeyword(kwSoulbond, "Soulbond") }
