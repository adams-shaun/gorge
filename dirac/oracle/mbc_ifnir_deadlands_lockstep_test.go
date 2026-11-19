package oracle

import "testing"

// Ifnir Deadlands' "{2}{B}{B}, {T}, Sacrifice a Desert" beside an Incubation
// Druid (a reflected Amount$ source, a census gap whose bound is {C} alone) and
// one other black source. gorge's potential walk prices the activation against
// the bound, which keeps the activating land's own {B}; the prefix search then
// pays {B}{B} with the Druid and the other source. The kernel's gaps walk
// dropped the land's own mana and never searched the play (tail-55 pair 163,
// decision 207).
func TestMBCIfnirDeadlandsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_gap_bound_counts_own_tap_source")
}
