package oracle

import "testing"

// CR 707.10c: a spell copy's controller may leave a target unchanged even when
// it is no longer legal. Katara's Reversal cast during Impulsivity's enters
// trigger targets that trigger; once it leaves the stack, Flare of
// Duplication's copy of the Reversal still offers it as a (departed) target.
// Gorge names it "-" (RefNone); the kernel offered its stale stack index S0
// (decision 890 of tail-62-c000 pair 8).
func TestMBCImpulsivityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_keeps_departed_stack_target")
}
