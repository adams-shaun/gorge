package oracle

import "testing"

// CR 603.2 with CR 119.4: City of Brass deals 1 damage to its controller each
// time it becomes tapped, so at 1 life gorge's planner never taps it for an
// activation's mana (the lethal guard). Wall of Brambles' {G}: Regenerate at
// 1 life beside an untapped City of Brass is then not offered. The kernel
// offered it on a table with a dynamic {T}, sacrifice source, because the
// activated-ability walk's availDynSac shared its bit with the cast window's
// availWinCol, which lifts the lethal guard (tail-57-c004 pair 112, decision 936).
func TestMBCWallOfBramblesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lethal_tap_pain_source_beside_dynamic_sac_walk")
}
