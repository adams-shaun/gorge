package oracle

import "testing"

// CR 305.6, 605.3a, 601.2g: Cabal Coffers with every land a Swamp (Realmwright's
// choice) carries the intrinsic "{T}: Add {B}" beside its printed "{2},{T}"
// paid filter. gorge and the kernel must tap the same lands to pay a cast beside
// it: the game once diverged at decision 712 with the kernel tapping the Coffers
// and gorge a Plains (tail-41 pair 0, seed 962358). The recording is the whole
// game, served whole, so it passes decision 712 and the casts after it.
func TestMBCCoffersTapChoiceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "coffers_intrinsic_tap_choice")
}
