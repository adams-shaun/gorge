package oracle

import "testing"

// CR 119.4 with CR 106.6: Untaidake, the Cloud Keeper's two restricted {C}
// cost 2 life, and the only white sources besides a Plains is a Scabland
// whose {W} deals 1 damage to its controller. At 3 life the only plan that
// pays Spider-Man, Peter Parker's {3}{W}{W} costs 3 life and would be lethal,
// so gorge's planner has no such plan and the cast isn't offered. The kernel
// offered it (decision 358 of tail-73-c000 pair 92, autopay).
func TestMBCSpiderManPeterParkerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_life_mana_with_pain_lethal")
}
