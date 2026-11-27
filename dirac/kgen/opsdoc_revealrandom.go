package kgen

// opsdoc_revealrandom.go: the OPS.md row for ExtRevealRandom.
func init() {
	opsEntries = append(opsEntries, opsDoc{
		"ext:RevealRandom",
		"OpExt ExtRevealRandom (Obj the player)",
		"A Reveal Random$ True over one target player's hand (gorge effReveal: Random$ narrows the hand pool to one card with a single seeded h.Rand(len) draw, then only a Note): the engine RNG draws once over that player's hand when it holds a card, and nothing else happens (no card moves, no decision). An empty hand or an Obj that resolves to no player draws nothing. Urza's Bauble.",
	})
}
