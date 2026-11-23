package oracle

import "testing"

// CR 605.3b, 601.2g: Holdout Settlement ({T}: Add {C}, or tap an untapped
// creature: any colour) beside School of the Unseen ({2},{T}: any colour) and
// Akoum Refuge. gorge's prefix search taps Holdout for {C} first and walks the
// whole dead subtree below it (School's {C}, every funding order), spending its
// 160-node budget before the creature-tap route that pays Baloth Prime's {G}
// is reached, so neither Baloth Prime nor Bountiful Harvest is offered. The
// kernel's free-branch prune read the board at the node, where the spent
// Holdout was no longer a tap-another gap, and skipped School's {C} subtree,
// reached the creature-tap route inside the budget and offered both (decision
// 274, tail-42 pair 10).
func TestMBCBalothPrimeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_free_prune_beside_tapx_gap")
}
