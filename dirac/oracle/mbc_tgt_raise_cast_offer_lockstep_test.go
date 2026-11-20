package oracle

import "testing"

// The dr-mbc-aaab evidence sweep's pair 94 (seed 222108), replayed on a table
// built from its own decks (pool "custom"): at p0's priority, gorge's autopay
// vocabulary carries only the casts its planner pays (Vanish into Eternity's
// own RaiseCost ValidTarget$ Creature declines the planner's shape, and its
// potential section is dropped) and the kernel must too. The kernel once
// offered the plain cast (gorge 6 casts, kernel 7).
func TestMBCVanishIntoEternityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tgt_raise_cast_offer")
}
