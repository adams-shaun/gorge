package oracle

import (
	"testing"
)

// CR 702.35b with 601.2e-g: a madness trigger's cast-or-graveyard ask prices
// the madness cost against gorge's potential bound (offerCastablePotential,
// rules/potential.go potentialManaFor), where an untapped open-production
// source (Endurance Bobblehead's {T}: Add one mana of any color) is unbounded
// in every colour, so the "cast it for its madness cost" choice is offered
// even when the floating pool and the exact window cannot pay 2BB -- a cast
// begun on it then aborts at the payment (CR 733). The kernel's
// madnessCastable priced the exact pool and window only, so it offered "no"
// alone (first divergence decision 356 of tail-32, Gisa's Bidding discarded
// from a Wheel and Deal).
func TestMBCGisaSBiddingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gisa_sbidding_optional_trigger_yes")
}
