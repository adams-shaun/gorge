package oracle

import "testing"

// Radiant Purge exiles a transformed Siege's crewed back face (Aetherwing,
// Golden-Scale Flagship) from the battlefield. CR 712.8a re-faces it front up
// in exile, but the card's transformed-cast offer (CR 310.12b) already fired
// when combat removed its last defense counter, and the owner took it: an
// external exile must not pose the offer again. The gorge side once queued
// the offer on any battlefield→exile move of a battle-shaped object with no
// defense counters, so it asked defeat_cast_yes/no where the kernel (which
// offers only on the defeat state-based action) kept passing. The recording
// is the first divergence of dr-ev-tail-10 pair 29.
func TestMBCRadiantPurgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_radiant_purge_defeat_cast_reoffer")
}
