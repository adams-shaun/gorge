package oracle

import "testing"

// The `projection Stack[N].Ability` class, Cryptoplasm's upkeep trigger: gorge
// and the dirac kernel project different ability identities for the stack item
// that carries Cryptoplasm's beginning-of-upkeep copy trigger. Recorded from
// the tail-34 evidence loop (pair 120, seed 361420, decision 970,
// implicating Rolling Hamsphere and Drowner of Secrets): Cryptoplasm's trigger
// goes on the stack at p0's upkeep and the next decision's projection reads
// gorge=128 kernel=255.
func TestMBCDrownerOfSecretsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cryptoplasm_upkeep_trigger_projection")
}
