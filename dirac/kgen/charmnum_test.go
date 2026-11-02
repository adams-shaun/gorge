package kgen

import "testing"

// TestCharmnumRefusals: CharmNum$/MinCharmNum$ are read as literal digits;
// Count$Compare / Count$Delirium counts lower (Let's Play a Game);
// every other count, and every target shape the kernel's combined
// per-mode target step does not read, is refused with a named reason.
func TestCharmnumRefusals(t *testing.T) {
	checkCarrier(t,
		[]string{"Pinecone Strike", "Craftwork Crusher", "Farewell", "Azula Always Lies", "Flash Thompson, Spider-Fan", "Rankle's Prank",
			"Epic Fight", "Go Nuts!", "HULK SMASH!", // a chain target announced after the combined ask
			"Trystan's Command", "Atlantis Attacks", "Amazing Acrobatics"}, // a 1..N slot, asked sequentially
		map[string]string{
			"Blessed Alliance":    "multi-pick charm mode target", // "up to two target players" (0..2)
			"Moment of Reckoning": "CanRepeatModes",
		},
		[]string{"Charm"})
}
