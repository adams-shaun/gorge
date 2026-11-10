package kgen

import "testing"

// TestCharmnumRefusals: CharmNum$/MinCharmNum$ are read as literal digits;
// Count$Compare / Count$Delirium counts lower, behind a bare SVar name too
// (Let's Play a Game, Drown in Dreams, Prophetic Titan), as does the paid
// additional cost and a general "up to V" count (Pyrrhic Strike, Bumi, the
// Ruinous-crew shape); every other count, and every target shape the
// kernel's combined per-mode target step does not read, is refused with a
// named reason.
func TestCharmnumRefusals(t *testing.T) {
	checkCarrier(t,
		[]string{"Pinecone Strike", "Craftwork Crusher", "Farewell", "Azula Always Lies", "Flash Thompson, Spider-Fan", "Rankle's Prank",
			"Epic Fight", "Go Nuts!", "HULK SMASH!", // a chain target announced after the combined ask
			"Trystan's Command", "Atlantis Attacks", "Amazing Acrobatics", // a 1..N slot, asked sequentially
			"Pyrrhic Strike", "Bumi, King of Three Trials", "The Ruinous Wrecking Crew", "Moment of Reckoning", // Moment: repeatable modes (charmrepeat_test.go)
			"Drown in Dreams", "Prophetic Titan"},
		map[string]string{
			"Blessed Alliance": "multi-pick charm mode target", // "up to two target players" (0..2)
		},
		[]string{"Charm"})
}
