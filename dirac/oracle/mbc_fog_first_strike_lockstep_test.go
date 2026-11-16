package oracle

import "testing"

// CR 510.4: a creature with first strike (and not double strike) deals combat
// damage only in the first-strike damage step. Kami of False Hope's "prevent
// all combat damage" meant the kernel skipped that first pass, never marked
// Ballynock Cohort (first strike, blocked by two creatures) as having dealt
// its damage, and asked its damage division again in the regular step, where
// gorge goes straight to priority (dr-mbc-bbaa pair 106, decision 211).
func TestMBCBallynockCohortReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_fog_first_strike_no_second_damage")
}
