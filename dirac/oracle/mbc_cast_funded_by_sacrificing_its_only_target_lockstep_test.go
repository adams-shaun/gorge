package oracle

import "testing"

// Waste Away ({4}{B}, discard a card, target creature gets -5/-5) with the
// caster's only creature on the table a Catalyst Elemental ("Sacrifice this:
// Add {R}{R}") that the {4} needs: the kernel offers the cast (the creature is
// chosen as the target, then sacrificed for mana; the spell fizzles, CR
// 601.2c, 601.2h, 608.2b) and gorge offered pass only (decision 248 of pool
// tail-12, surface full).
func TestMBCWasteAwayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_funded_by_sacrificing_its_only_target")
}
