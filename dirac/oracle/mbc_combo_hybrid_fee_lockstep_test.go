package oracle

import (
	"testing"
)

// CR 106.1, CR 605.3a: a second {U} activation of Dukhara Peafowl with the
// Island tapped is payable through Energy Refractor's repeatable "{2}: Add one
// mana of any color" (fee: Mystic Gate's {C} and a Plains' {W}). gorge's
// census (paymentPlanCensusOf) read that fee as the prefix search's funding
// only while no earlier gap had made the census unrelaxable and unbounded, so
// Mystic Gate's hybrid-fee ability ahead of the Refractor in zone order left
// the search unfunded and the activation withheld, while dirac's
// filtFunding offered it (ev-tail-02-c000 pair 131).
func TestMBCDukharaPeafowlReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combo_hybrid_fee")
}
