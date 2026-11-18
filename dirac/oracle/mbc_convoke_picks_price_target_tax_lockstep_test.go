package oracle

import "testing"

// CR 601.2c with CR 601.2f and CR 702.51a: Stoke the Flames (Convoke) with
// three creatures already announced owes {1} of its printed cost, and an
// opponent's Syr Elenora, the Discerning raises a spell that targets her by
// {2}. Two untapped lands and two Eldrazi Scion tokens pay the {3}, so gorge
// offers Elenora as a target. The kernel once priced the candidate at the
// whole {4}{R}{R}, ignoring the announced convoke picks, and withheld her
// (decision 540 of 540; tail-14-c000 pair 4, seed 915732).
func TestMBCSyrElenoraConvokePicksReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_picks_price_target_tax")
}
