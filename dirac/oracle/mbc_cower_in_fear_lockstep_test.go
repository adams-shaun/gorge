package oracle

import "testing"

// CR 702.73a with CR 603.10a: three Shapeshifter changeling tokens die to
// Cower in Fear while two A-Shipwreck Sifters watch for a Spirit put into
// their controller's graveyard. A dying changeling is still every creature
// type, so both Sifters trigger and the kernel asks for their order. Gorge
// once dropped the changeling's all-types marker from its departure type
// snapshot whenever a layer-4 effect was in play, saw no Spirit, and never
// asked (decision 651 of tail-36-c000, pair 3, seed 980429).
func TestMBCCowerInFearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "changeling_token_death_spirit_watcher")
}
