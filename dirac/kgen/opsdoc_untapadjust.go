package kgen

// opsUntapAdjust is the OPS.md row for Static Orb's player keyword.
func init() {
	opsEntries = append(opsEntries, opsDoc{"stat:UntapAdjust", "StatUntapAdjust (Filter, A, Cond2)",
		"During each player's untap step, at most A permanents matching Filter untap, and that player chooses which of the tapped ones stay tapped (S:Mode$ Continuous | Affected$ Player | AddKeyword$ UntapAdjust:Permanent:2, CR 702.19e's \"can't untap more than N permanents\" family). The kernel reads each live static under Cond2 (Static Orb's IsPresent$ Card.Self+untapped) in untapStepGen and poses the cap as one ChGenKeepTapped multi-pick of the tapped candidates, marking each chosen one pfSkipUntap so the resumed scan leaves it tapped; the gorge reader is rules/untap.go untapAdjustCap and turn.go finishUntapStep's cap election. Only the Permanent filter is lowered; the Land/Creature/Artifact printings (Winter Orb, Smoke, Imi Statue) stay refused. Static Orb."})
}
