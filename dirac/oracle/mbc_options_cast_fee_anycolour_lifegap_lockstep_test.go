package oracle

import "testing"

// CR 601.2h with CR 603.2: casting Creeperhulk ({3}{G}{G}) stays legal when the
// only green the plain census prices is an enchanted land whose tap fires an
// unvetted LoseLife watcher (Corrupted Roots defers the Forest on both
// engines) and a restricted artifact sits untapped beside it: gorge's prefix
// search prunes Myr Reservoir's {C}{C} ("Spend this mana only to cast Myr
// spells", CR 106.6) as a dead funding branch and reaches the plan -- the
// deferred Forest's {G} beside Hall of Tagsin's "{1},{T}: add one mana of any
// colour" fee and plain generics -- within the node budget. The kernel once
// walked the dead branch, exhausted its budget and refused the cast
// (pair 10 of pool tail-39-c000, seed 525523779106261957).
func TestMBCCreeperhulkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "options_cast_fee_anycolour_lifegap")
}
