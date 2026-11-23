package oracle

import "testing"

// CR 602.2b, 605.3a: Sokenzan, Crucible of Defiance's channel ({3}{R}, discard
// it) from the hand is payable with five lands when Crossroads Candleguide's
// "{2}: add one mana of any color" makes the red, so gorge offers it; the
// kernel's plain census, which holds no fee-paid red unit, did not
// (tail-26-c000 pair 28, decision 312).
func TestMBCSokenzanCrucibleOfDefianceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hand_channel_fee_source_offered")
}
