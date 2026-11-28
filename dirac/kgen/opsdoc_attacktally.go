package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ex:CardNumAttacksThisTurn", "ExCardNumAttacksThisTurn",
			"The times the evaluating source was declared as an attacker this turn, every combat of the turn counted (gorge Count$CardNumAttacksThisTurn, state.Object.AttacksThisTurn: the DeclareAttackers fold increments it, TurnChange resets it). Under an AffectedX static (kop.StPerAffected) the source is each affected creature. A per-object u8 cold tally, counted before the declaration's triggers and cleared at turn start. dirac/value_attacktally.go. Moraug, Fury of Akoum."},
	)
}
