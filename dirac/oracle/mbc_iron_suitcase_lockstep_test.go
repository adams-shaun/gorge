package oracle

import "testing"

// Iron Suitcase's "{3}: becomes a 3/3 flying creature" activation beside a
// Mercadian Bazaar (a storage battery whose removal cost announces X).
// Repro: pool tail-66, pair 140 (autopay).
func TestMBCIronSuitcaseReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_funds_a_costly_activation")
}
