package oracle

import "testing"

// CR 702.138a: gorge's planner prices no escape cast (potentialModeBaseCost
// has no arm), so its potential walk lists Fruit of Tizerus's escape cast as
// "unsupported shape", never an insufficient play a prefix search funds. The
// kernel's script search took the escape cast from the any-mana walk and
// offered it once a storage battery (Dwarven Hold) completed {3}{B} with its
// storage counter (tail-08 pair 9, decision 391).
func TestMBCFruitOfTizerusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "escape_cast_not_script_searched")
}
