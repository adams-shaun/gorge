package oracle

import "testing"

// CR 601.2g with CR 605.3c: a repeatable mana ability whose activation cost taps
// another permanent (Urza, Lord High Artificer's "Tap an untapped artifact you
// control: Add {U}") adds one unit per untapped candidate to the mana a cast can
// draw on, so Marang River Prowler's {2}{U}{B}-class cast beside Urza and two
// untapped artifacts (Jeweled Lotus's commander-only mana refused by this
// payment) is payable and must be offered. gorge's potential bound counted the
// source once and withheld the cast the kernel offered (decision 1092 of the
// tail-64-c000 pair 16 game the evidence loop parted on); the bound now folds
// the repeats (engine/rules potential_repeat_fee.go).
func TestMBCMarangRiverProwlerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapanother_repeat_potential_cast_offer")
}
