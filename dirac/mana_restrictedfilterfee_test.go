package dirac

import "testing"

// Hydraulic Helper's restricted {U} (artifact spells only) still funds the
// {1} of Surveillance Room's any-colour ability: a mana ability is an
// activated ability, not a spell (CR 106.6, 602.2b).
func TestRestrictedSourceFundsFilterFee(t *testing.T) {
	for _, withHelper := range []bool{true, false} {
		sc, id := stdMshScen(t, "Trickster's Stratagem", "Surveillance Room", "Hydraulic Helper", "Swamp", "Red Room Recruit")
		for i := 0; i < 3; i++ {
			sc.perm(id["Swamp"], 0)
		}
		sc.perm(id["Surveillance Room"], 0)
		if withHelper {
			sc.perm(id["Hydraulic Helper"], 0)
		}
		sc.perm(id["Red Room Recruit"], 1)
		strat := sc.hand(id["Trickster's Stratagem"], 0)
		d := sc.s.Step()
		// Offered exactly when Helper is on the battlefield; without it the
		// cast is not payable.
		if got := castOffered(&d, strat); got != withHelper {
			t.Errorf("withHelper=%v: cast offered=%v, want %v", withHelper, got, withHelper)
		}
	}
}
