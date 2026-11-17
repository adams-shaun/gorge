package oracle

import "testing"

// CR 702.89a: printed Umbra armor is CardDef.UmbraArmor, a replacement of the
// enchanted permanent's destruction (kgen lower_umbraarmor.go; kernel
// kw_umbraarmor.go; gorge effects.ReplaceUmbraArmor), not a battlefield
// characteristic, so the chars projection lists it in gorgeKwElsewhere like
// Dredge and Wither. Before it did, Mammoth Umbra entering the battlefield was a
// projection error (decision 848 of tail-46 pair 0: gorge keyword "Umbra armor"
// has no kernel bit). The recording is pool "custom" (a table built from its own
// decks): a tail-NN bucket moves whenever the lowered set grows, and the replay
// then fails with a card "not in the pool" before it plays a decision.
func TestMammothUmbraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "umbra_armor_kernel_bit")
}
