package oracle

import "testing"

// Signpost Scarecrow's "{2}: Add one mana of any colour" is a repeatable
// fee-only mana ability (CR 605.3a, 602.2b), so a payment can fund a pip no
// land makes by paying its fee from tapped lands. Warhost's Frenzy's kicker
// is {B}, and on a red-and-white board with the untapped Scarecrow the kicked
// {2}{R}+{B} cast is payable that way (pool tail-45 pair 36, seed 585512,
// decision 472: gorge's options alone carried cast(#38,-,1)). The kernel's
// phasedFallback now walks the Taps-deferred gap beside a repeatable fee-only
// filter as gorge does, so both menus carry the kicked cast and the recorded
// game replays without a divergence.
func TestMBCWarhostSFrenzyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_fee_anycolour_funds_kicker")
}
