package oracle

import "testing"

// CR 702.33a / 702.56a: a spell cast with Replicate paid is not a kicked spell,
// so Bloodstone Goblin ("whenever you cast a spell, if that spell was kicked")
// doesn't trigger on Pyromatics. dirac once read the replicate count's kick
// flag as kicked and offered Bloodstone Goblin's order option in the cast
// triggers' ordering decision (decision 253, tail-40-c000 pair 15).
func TestMBCBloodstoneGoblinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "replicate_cast_is_not_kicked")
}
