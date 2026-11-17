package oracle

import "testing"

// CR 601.2b, 601.2h: Knollspine Invocation's "{X}, Discard a card with mana
// value X" announces X before the discard is chosen, so after X = 1 only the
// mana-value-1 card pays it. The gorge side read X as 0 off the not-yet-cast
// ability and offered the hand's mana-value-0 cards (Mobilized District,
// Mountain) instead of Soul's Attendant (tail-53 pair 0, decision 215).
func TestMBCMobilizedDistrictReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "discard_cost_cmc_x_binds_announced_x")
}
