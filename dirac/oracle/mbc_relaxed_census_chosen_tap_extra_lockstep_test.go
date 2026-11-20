package oracle

import "testing"

// CR 605.4a: tapping a land with a Utopia Sprawl chosen-{U} on it adds {G}
// and, to the same player, the Sprawl's {U}. Mantle of Tides' Equip {3} with
// an Izzet Signet's {1} fee beside it is payable from that tap plus the
// Signet's {U}{R}. gorge's relaxed census (potential_plan.go
// paymentPlanRelaxedAlternatives) read the deferred tap without the chosen
// extra, proved the activation unpayable and never ran its prefix search,
// while the kernel's script route floated the choice at the tap and offered
// it (decision 774 of 774, tail-47 pair 6 seed 866479).
func TestMBCMantleOfTidesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "relaxed_census_chosen_tap_extra")
}
