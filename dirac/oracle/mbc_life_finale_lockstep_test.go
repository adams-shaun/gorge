package oracle

import "testing"

// CR 701.23 with CR 115.1: Life's Finale's "search target opponent's library"
// searches the targeted opponent's library, though a chained sub-ability's
// target is what names it (tail-04-c000 pair 5, decision 579). Gorge offered
// the caster's own creature cards where the kernel offered the opponent's.
func TestMBCBackupAgentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_finale_searches_targeted_opponents_library")
}
