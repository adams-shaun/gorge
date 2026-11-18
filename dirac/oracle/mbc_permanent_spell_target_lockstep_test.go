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
	// CR 608.2n with CR 707.10 and CR 704.5d: a copy of Beacon of Immortality
	// (copied by Lithoform Engine) is shuffled into its owner's library with the
	// library, then ceases to exist, so the shuffle permutes one card more than
	// the library held. dirac shuffled the library alone, one RNG draw short
	// (RNGDraws ref=131 kernel=130 at decision 1123, tail-15-c000 pair 11).
	stdReproNamed(t, "spell_copy_shuffled_into_library")
}
