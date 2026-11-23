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
//
// The recording also plays the offer: it picks the activation at decision 482,
// answers the two {G/U} pip announcements (decisions 483 and 484: blue, then
// green) and passes until the ability resolves (485 and 486). The autopay
// surface pays with no manual window ask, and the lockstep compares both
// engines' taps, fee activations and produced colours through it. stdReproNamed
// fails if any recorded action is skipped, so a drifted recording cannot pass
// as a different game.
func TestMBCShamanOfTheGreatHuntReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, actFeeAnyColourRepro)
}
