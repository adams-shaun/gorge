package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"pred:KickedAlt", "PredKickedAlt (PredN 1 / 2)",
			"A battlefield permanent cast with kicker alternative PredN paid (gorge `Card.Self+kicked 1` / `kicked 2`, FlagKicked1 / FlagKicked2). Reads bit PredN-1 of the object's coldKickAlt byte (dirac/filter_kickedalt.go), which only setKickAlt writes and beginCast, a non-spell entry and an object-id reuse clear: a single-cost kicked cast sets no bit, so both indexes read false (gorge's index forms ignore the bare FlagKicked). Never matches off the battlefield (kgen refuses it there) except as a SpellCast self trigger's filter, read off the spell's own byte at cast time (selfCastFilterOK). A two-alternative Kicker card's self-ETB and self-cast triggers."})
}
