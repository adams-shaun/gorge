package oracle

import "testing"

// CR 602.2 with CR 118.3: Barbarian Ring's "{R}, {T}, Sacrifice" threshold
// ability can't be paid by the Ring's own {T}: add {R}. With the controller at
// 2 life the Ring's painful {R} sits in painTight's pain group, which the
// source-removal left standing, so dirac offered the activation gorge (rightly)
// did not (decision 316 of tail-54-c018 pair 2, replayed on its own decks' table).
func TestMBCBarbarianRingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pain_only_source_leaves_its_own_tight_group")
}
