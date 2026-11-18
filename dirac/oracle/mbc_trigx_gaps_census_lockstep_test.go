package oracle

import (
	"testing"
)

// Taj-Nar Swordsmith's "you may pay {X}" announces X up to the most mana its
// payer can produce (CR 107.3i, 601.2b), counting every mana ability usable
// now, including one whose cost taps nothing. The kernel's capped census
// (availForCapped) read tap- and sacrifice-cost sources only and offered only X = 0
// where gorge offers 0, 1 and 2. The recorded game's first divergence is the
// kernel's missing X = 1 and X = 2 options at decision 672 (options options,
// tail-55-c000 pair 87); the same class as trigx_ctr_census.
func TestMBCTajNarSwordsmithReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigx_gaps_census_taj_nar")
}
