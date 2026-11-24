package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ex:SpellsCastLastTurn", "ExSpellsCastLastTurn (N)",
			"The spells cast in the turn before this one, whoever's it was, every player's casts counted: N 0 whether any player cast a spell (0 or 1), N 1 the most one player cast (saturating at 3). beginTurn rolls the history bank's per-seat spell tallies into the dayNight byte (bits 4-6) before the bank resets. The Innistrad werewolves' intervening-ifs: WerewolfTransformCondition$ is the N 0 count == 0 (\"if no spells were cast last turn\"), WerewolfUntransformCondition$ the N 1 count >= 2 (\"if a player cast two or more spells last turn\"); gorge trigmatch phaseTriggerMatches over SpellsCastLastTurn. dirac/value_spellslastturn.go. Daybreak Ranger, Gatstaf Shepherd, Hanweir Watchkeep, Huntmaster of the Fells, Mayor of Avabruck, Call of the Full Moon."})
}
