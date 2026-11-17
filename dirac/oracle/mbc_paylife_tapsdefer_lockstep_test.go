package oracle

import "testing"

// TestMBCBlindblastReprosLockstep: under Price of Glory, p1's lands tapped on
// the opponent's turn are Taps-deferred (the watcher's Destroy body), and
// Ramunap Ruins' "{T}, Pay 1 life: Add {R}" is p1's only red source. gorge
// offers the red instants (Blindblast, Bombard, Crimson Wisps) funded by it;
// the kernel offered only pass (dr-mbc-abba pair 73, decision 236).
func TestMBCBlindblastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paylife_mana_source_under_tap_watcher")
}
