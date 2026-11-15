package oracle

import "testing"

// CR 605.3a: the kernel's filter funding pass refused a sacrifice-only Eldrazi
// Spawn as a fee source (it has no tap), so with the pool empty it priced
// Shimmering Grotto's {1},{T} fee as unfundable and withheld Towering
// Thunderfist's "{W}: gains vigilance" where gorge offered it (decision 371 of
// 371; tail-10 pair 1555).
func TestMBCSacOnlySpawnFundsFilterFeeLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_spawn_funds_filter_fee")
}
