package oracle

import "testing"

// TestMBCFreeCastTaxPickReprosLockstep replays the synthetic tail-64 game where
// Urza, Lord High Artificer's "you may play that card without paying its mana
// cost" casts Culling the Weak (additional cost: sacrifice a creature) under
// Thorn of Amethyst's {1} tax. gorge's CR 601.2g window taps the tax's source
// before the sacrifice is chosen (manaWindowAsk before costObjectAsks, CR
// 601.2h); the kernel kept a free cast's whole payment for after the pick, so at
// the sacrifice decision gorge's Island was tapped with {U} floating and the
// kernel's was not (divergence at decision 586: projection Cards[14].Flags).
func TestMBCFreeCastTaxPickReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_tax_pick_window_first")
}
