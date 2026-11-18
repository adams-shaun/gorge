package oracle

import "testing"

// CR 508.1: declaring attackers. Warden of the Chained ("Warden of the
// Chained can't attack or block alone" — see the card's own restriction) sits
// on a battlefield where it is the only legal attacker for its controller: the
// other creatures are missing, tapped, sick, or forbidden, so the restriction
// that binds it also can't be satisfied and the empty declaration is legal.
// Dirac still offers attack(warden), so the two engines part at the attackers
// decision's option list (decision 378 of tail-16-c000 pair 31, seed 182750).
func TestMBCWardenOfTheChainedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "warden_of_the_chained_attackers_options")
}
