package oracle

import "testing"

// CR 601.2c with 601.2f: Join Forces ("up to two target creatures") against
// Charix, the Raging Isle's {2} tax on spells that target it. The empty
// answer leaves that target-dependent cost to be paid from what the caster
// holds, which includes the floating restricted mana the spell may spend.
// Dirac priced the empty answer from the plain pool and untapped sources
// alone, judged it unpayable and withheld `done`, where gorge offers it
// (decision 452, tail-34 pair 47 seed 851339).
func TestMBCJoinForcesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "empty_target_set_counts_restricted_mana")
}
