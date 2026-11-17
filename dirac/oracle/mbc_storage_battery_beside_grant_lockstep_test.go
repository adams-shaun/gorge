package oracle

import "testing"

// CR 601.2h, CR 605.3a: p0 holds Barrel Down Sokenzan ({2}{R}) beside three
// Plains, a Mercadian Bazaar with storage counters (its {R} comes from an
// announced X, so no planner unit) and a creature wearing Paradise Mantle
// (its granted {T} mana ability is a V1 exclusion of gorge's planner). Gorge's
// planner prices no payment for the cast and the pool is empty, so the autopay
// surface leaves the plain cast out; the kernel's grant search put it back
// because a prefix through the storage battery pays it (decision 504 of
// tail-26-c000 pair 53).
func TestMBCBarrelDownSokenzanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_beside_grant_plain_cast_dropped_by_autopay")
}
