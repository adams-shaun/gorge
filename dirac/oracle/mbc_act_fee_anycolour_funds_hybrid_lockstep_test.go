package oracle

import "testing"

const actFeeAnyColourRepro = "act_fee_anycolour_funds_hybrid"

// Signpost Scarecrow's "{2}: Add one mana of any colour" is a repeatable
// fee-only mana ability (no {T}, no limit), so a payment activates it as often
// as it can fund the fee (CR 605.3a, 602.2b). Shaman of the Great Hunt's
// Ferocious activation, {2}{G/U}{G/U}, is therefore legal on a red-and-white
// board with the untapped Scarecrow: the tapped lands pay the two {2} fees and
// the activation's {2}, and the Scarecrow's two activations make the {G/U}
// pips (pool tail-45 pair 41, seed 980704, decision 482). Gorge found it once
// its plain prefix search spent the node budget on the Scarecrow's fee
// branches and its phased run (free gaps, covered funding, fee gaps) took
// over, with a Taps-deferred source on the board among the gaps. The kernel's
// phased fallback (dirac/mana_filter.go phasedFallback) used to stand down on
// any other gap, so only gorge offered the activation. With the Taps-deferred
// gap walked in the phased run's first phase both engines offer it and the
// recorded game replays clean; the card is no longer refused
// (dirac/support/unrefused.tsv) and this test pins the unrefusal's proof.
func TestMBCShamanOfTheGreatHuntReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, actFeeAnyColourRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", actFeeAnyColourRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", actFeeAnyColourRepro, res.Stall)
	}
}
