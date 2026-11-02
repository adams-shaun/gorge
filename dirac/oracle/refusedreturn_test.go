package oracle

import "testing"

// TestReturnedCardsLockstep plays, in lockstep, the pairs that showed each
// difference whose cause was fixed: the cards are no longer refused by name,
// so each pair runs clean on its own surface.
func TestReturnedCardsLockstep(t *testing.T) {
	autopay, full := Config{GorgeAutopaySurface: true}, Config{}
	for _, tc := range []struct {
		name  string
		pool  string
		cfg   Config
		mull  int
		pairs []int
	}{
		// Metamorphic Blast and Phantom Interference: a Spree mode gate with a may-play rider
		{"metamorphic blast", "std-otj", autopay, 0, []int{442}},
		{"phantom interference", "std-otj", autopay, 1, []int{1292}},
		// Pitiless Carnage: an optional sacrifice leaves as one batch
		{"optional sacrifice batch", "std-otj", autopay, 0, []int{498}},
		// Resurrected Cultist: loses all abilities against a become-blocked trigger
		{"blocked trigger without abilities", "std-dsk", autopay, 1, []int{398}},
		// Smile at Death: a once-each-turn filter mana source under CantBeActivated
		{"smile at death", "std-tdm", full, 1, []int{1379}},
		// Victor Valgavoth's Seneschal and Tainted Treats, whose earlier differences no longer show
		{"victor valgavoth's seneschal", "std-dsk", autopay, 0, []int{1488}},
		{"tainted treats", "std-tmt", full, 0, []int{1463}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synthPairsMulligans(t, tc.pool, RunConfig{Config: tc.cfg}, tc.mull, tc.pairs, nil)
		})
	}
}
