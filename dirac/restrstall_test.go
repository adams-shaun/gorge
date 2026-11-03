package dirac

import "testing"

// TestFilterSearchOverAThirdRestrictionStalls: poolR names two restrictions
// (genPrep.restrF), so with Villainous Hideout's Villain-only {G} beside
// Avengers Tower's Hero-only one and Castle Doom's artifact-only one, the
// Hideout's restricted mana floats unrestricted (a third kind). A filter
// search that funds Claim the Kingdom's {1}{G} through it would offer a cast
// gorge's planner never prices (the mana cannot be spent on it, CR 106.6),
// so the kernel latches the capacity stall rather than offer the play
// unflagged.
func TestFilterSearchOverAThirdRestrictionStalls(t *testing.T) {
	sc, id := synthScen(t, "std-msh", "Claim the Kingdom", "Villainous Hideout", "Surveillance Room", "Plains", "Avengers Tower", "Castle Doom")
	sc.perm(id["Villainous Hideout"], 0)
	sc.perm(id["Surveillance Room"], 0)
	c := sc.hand(id["Claim the Kingdom"], 0)
	sc.castOptions(c)
	if !sc.s.Stalled() {
		t.Fatal("a script funded by a third restriction's mana did not latch a capacity stall")
	}
}
