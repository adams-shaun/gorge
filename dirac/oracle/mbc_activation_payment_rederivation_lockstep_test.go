package oracle

import "testing"

// TestMBCActivationPaymentRederivationReprosLockstep replays two synthetic
// tail-49 games where gorge's bot paid a non-mana activation with elided mana
// activations and the kernel's re-derivation of that payment at the
// activation's Apply floated and tapped differently.
//
// Tamiyo's Logbook ({5}{U}, {T}; "ReduceCost$ X" for each other artifact) was
// priced at its printed {5}{U} by the prefix payment (abPlannedCost skipped the
// ability's own reduction that gorge's planner folds, CR 601.2f), so the kernel
// tapped two more Islands and floated {G}{G} (divergence at decision 532:
// projection Cards[15].Flags, Pool[0][4]).
//
// Arc Mage's {2}{R}, {T}, discard is funded through Relic of Legends' "Tap an
// untapped legendary creature" ability: that float covers the red pip and the
// Mountains pay the rest after the target ask, as gorge's search stops at the
// node the planner prices; the kernel floated one more Mountain (divergence at
// decision 526: projection Cards[19].Flags, Pool[0][3]).
func TestMBCActivationPaymentRederivationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activation_own_reduce_prices_prefix_payment")
	stdReproNamed(t, "activation_tapx_priced_remainder_floats_no_more")
}
