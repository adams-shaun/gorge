package oracle

import "testing"

// CR 118.3, 107.3, 603.12: Quantum Entanglement's "you may pay {1}{W}" end-step
// trigger beside an Icatian Store holding storage counters and no other
// untapped source. gorge poses "pay" (the battery is a source while it holds a
// counter, windowSourceYields), then funds it through the manual window, whose
// activation announces X at the bound (all the counters). The kernel once
// offered decline only, and once it offered "pay" it asked X instead of taking
// the bound (decision 415 of pair 5, pool tail-42, surface autopay; recorded on
// a custom pool because a synthetic tail pool's membership moves with the
// corpus). It now takes the bound unasked, as gorge's non-interactive
// ManaCostSubCounterStage does.
func TestMBCQuantumEntanglementReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_battery_source_asks_x")
}
