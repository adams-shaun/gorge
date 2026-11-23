package oracle

import "testing"

// CR 115.3 with CR 603.2: Shimmering Glasskite's "first time each turn" trigger
// fires once for one spell that names it more than once: Seeds of Strength
// targets the Glasskite in two of its three clauses, and the Glasskite is one
// targeted object, so one counter trigger queues, not one per clause. Gorge
// once queued one per TargetsChosen event and offered a second trigger_order
// option (decision 838 of tail-14 pair 75).
func TestMBCShimmeringGlasskiteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "becomes_target_first_time_same_spell_twice")
}
