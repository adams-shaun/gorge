package oracle

import "testing"

// CR 107.4f, 119.4: Thundering Tanadon ({4}{G/P}{G/P}) at one life can pay a
// pip only with {G}, so the cast needs {4}{G}{G}: six mana. The seat holds
// three Swamps, a Forest and an untapped Gemstone Mine whose mining-counter
// mana ability the cast window's census leaves unpriced, and the census's
// fail-open kept the plan (decision 765, tail-28-c000 pair 7, seed 442252).
// The incomplete census cannot prove a cast payable, but its unpriced
// production still has a ceiling; a cost past that ceiling is unpayable.
func TestMBCThunderingTanadonLowLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_cast_pips_unpayable_at_low_life")
}
