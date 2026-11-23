package oracle

import "testing"

// TestMBCLilianaSContractReprosLockstep replays the synthetic tail-42 game
// the evidence loop recorded: p0's upkeep (a Vedalken Orrery's "you may cast
// spells as though they had flash" in play) holds priority over two
// activations with Liliana's Contract ({3}{B}{B}) in hand and five mana in
// Subterranean Hangar's "{T}, remove X storage counters: Add {B} for each"
// (2 counters), Akoum Refuge, a Forest and School of the Unseen. gorge's
// planner prices the cast: its census admits the storage battery's bounded
// production and holds Gwenna, Eyes of Gaea's creature-only mana out
// (potentialRestrictionAdmits), and its relaxed proof pays through the
// empty-filter subset (paymentPlanRelaxProof accepts any subset of the paid
// gaps). The kernel's wide-source omit arm dropped the cast without a
// search: its held-out walk was the fee-charged gaps walk alone
// (School of the Unseen's {2},{T} fee is charged there), so the
// empty-filter subset that pays the cast was never consulted
// (divergence at decision 737: options options, only gorge offered the cast).
func TestMBCLilianaSContractReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "wide_omit_ignores_nofilt_subset")
}
