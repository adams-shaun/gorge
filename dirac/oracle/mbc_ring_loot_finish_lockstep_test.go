package oracle

import "testing"

// The Ring emblem's level-2 loot with an empty hand: the discard needs no
// ask, so ringLootDiscard used to finish the delayed resolution itself and
// resolveTop finished it again, wrapping the stack pointer. Recorded from
// tail-71 pair 29, decision 777 (kernel panic in projRest, implicates
// Hateflayer, the Ring-bearer whose attack fired the loot).
func TestMBCRingLootFinishReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_loot_double_finish")
}
