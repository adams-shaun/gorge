package oracle

import "testing"

// CR 110.4b: a "permanent spell" is a spell on the stack that will
// become a permanent, so Lithoform Engine's third ability ({4}, {T}: copy
// target permanent spell you control) has a legal target in the creature
// spell its controller just cast. Gorge read the script's `Permanent.YouCtrl`
// as a battlefield-only spec on the stack and withheld the activation that
// dirac offered (tail-53 pair 15, decision 296: gorge [pass], kernel
// [pass activate(Lithoform Engine, ability 2)]).
func TestMBCLithoformEngineReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permanent_spell_is_a_valid_stack_target")
}
