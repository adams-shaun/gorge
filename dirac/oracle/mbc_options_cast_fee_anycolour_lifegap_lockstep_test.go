package oracle

import "testing"

// CR 601.2h with CR 603.2: casting Creeperhulk ({3}{G}{G}) stays legal when the
// only green the census never prices is an enchanted land whose tap fires an
// unvetted LoseLife watcher (Corrupted Roots defers the Forest on both
// engines): the plan pays the two G from Caves of Koilos' "{T}, pay 1 life:
// add one mana of any colour" and Hall of Tagsin's "{1},{T}: add one mana of
// any colour" fee unit beside plain generics. The kernel once refused the cast
// (pair 10 of pool tail-39-c000, seed 525523779106261957).
func TestMBCCreeperhulkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "options_cast_fee_anycolour_lifegap")
}
