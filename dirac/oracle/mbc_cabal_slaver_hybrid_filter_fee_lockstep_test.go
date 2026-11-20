package oracle

import "testing"

// CR 601.2g-h and 605.3b: Fetid Heath's hybrid-fee mana ability ("{W/B}, {T}:
// Add {W}{W}, {W}{B} or {B}{B}") can make the black mana for Cabal Slaver's
// {3}{B}{B} only after another source has floated a {W} to pay the fee. Gorge's
// autopay census left that fee unfunded, so at priority decision 246 of the
// tail-01 pair 99 recording gorge offered only pass while the kernel offered
// Cabal Slaver, Pride of the Perfect and Restless Dreams. The three casts were
// listed as cast/unsupported census_incomplete in gorge's unpriced plays.
func TestMBCCabalSlaverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cabal_slaver_hybrid_filter_fee")
}
