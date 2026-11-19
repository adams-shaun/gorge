package oracle

import "testing"

// CR 608.2h and 202.3e: Dovescape counters a noncreature spell and its
// controller creates X Birds, X being that spell's mana value. A Ravaging
// Blaze cast with X=2 had mana value 4 on the stack, so four Birds; gorge read
// the value after the Counter had moved the spell to the graveyard, where X is
// 0, and made two (decision 625 of tail-22-c000 pair 37, projection NInst).
func TestMBCDovescapeXSpellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dovescape_counter_x_spell_mana_value")
}
