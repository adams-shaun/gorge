package dirac

import "testing"

// TestEachPotentialCountsEachRuneOnce: a lone Birds of Paradise rewritten to
// the all-at-once "{T}: Add {W}{U}" has no TapsForMana carrier on the board,
// so the over-bound potential pool holds exactly its two runes, one {W} and
// one {U}: gorge's PotentialMana sums each rune once. The all-at-once rest
// (pcard.manaXv) is already in the tap's colour mask, and the tap-extra fold
// (potentialTapExtra) must not price it a second time.
func TestEachPotentialCountsEachRuneOnce(t *testing.T) {
	sc, _, birds := eachStallBoard(t, false, 1, 1)
	if !sc.s.scriptGaps(sc.k, 0).has(birds) {
		t.Fatalf("precondition: the rewritten Birds are no each-colour gap")
	}
	if !sc.k.manaEach {
		t.Fatalf("precondition: the table has no all-at-once source")
	}
	var pot avail
	sc.s.availPotential(sc.k, 0, &pot)
	if pot.total != 2 {
		t.Fatalf("potential total %d (cnt %v), want 2: one {W} and one {U} rune", pot.total, pot.cnt)
	}
}
