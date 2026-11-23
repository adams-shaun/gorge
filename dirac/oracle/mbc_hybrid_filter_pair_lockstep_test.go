package oracle

import "testing"

// CR 605.3a-b: Slippery Karst's {G} funds Flooded Grove's "{G/U}, {T}: Add
// {U}{U}" fee, and its {U} pays Roil Cartographer's {1}{U} beside a second
// hybrid filter land (Wooded Bastion). gorge offers the cast; dirac's offer
// gate did not (kfuzz tail-15-c000 pair 62, decision 745).
func TestMBCRoilCartographerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_pair_cast_offer")
}
