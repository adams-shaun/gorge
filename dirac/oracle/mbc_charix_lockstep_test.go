package oracle

import "testing"

// CR 601.2b/601.2f/601.2h: Charix, the Raging Isle taxes spells that target it, so a
// target decision for a spell the caster can afford only without the tax is where the
// two engines part (decision 632 of tail-34-c000 pair 1: Sever the Bloodline).
func TestMBCCharixTheRagingIsleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_taxed_creature_beyond_mana")
}
