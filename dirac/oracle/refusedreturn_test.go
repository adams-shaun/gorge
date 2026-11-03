package oracle

import "testing"

// TestReturnedCardsLockstep plays, in lockstep, the pairs that showed each
// difference whose cause was fixed: the cards are no longer refused by name,
// so each pair runs clean on its own surface.
func TestReturnedCardsLockstep(t *testing.T) {
	autopay, full := Config{GorgeAutopaySurface: true}, Config{}
	for _, tc := range []struct {
		name string
		pool string
		cfg  Config
		mull int
		card string // a card the pairs must cast or enter
	}{
		// Metamorphic Blast and Phantom Interference: a Spree mode gate with a may-play rider
		{"metamorphic blast", "std-otj", autopay, 0, "Metamorphic Blast"},
		{"phantom interference", "std-otj", autopay, 1, "Phantom Interference"},
		// Pitiless Carnage: an optional sacrifice leaves as one batch
		{"optional sacrifice batch", "std-otj", autopay, 0, "Pitiless Carnage"},
		// Resurrected Cultist: loses all abilities against a become-blocked trigger
		{"blocked trigger without abilities", "std-dsk", autopay, 1, "Resurrected Cultist"},
		// Smile at Death: a once-each-turn filter mana source under CantBeActivated
		{"smile at death", "std-tdm", full, 1, "Smile at Death"},
		// Mockingbird: a flickered clone's election, and an {X} spell that reads its mana spent paid through a filter land
		{"mockingbird", "std-blb", full, 0, "Mockingbird"},
		// Victor Valgavoth's Seneschal and Tainted Treats, whose earlier differences no longer show
		{"victor valgavoth's seneschal", "std-dsk", autopay, 0, "Victor, Valgavoth's Seneschal"},
		{"tainted treats", "std-tmt", full, 0, "Tainted Treats"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinnedGamesRun(t, tc.pool, RunConfig{Config: tc.cfg}, tc.mull, cardOut(tc.card))
		})
	}
}
