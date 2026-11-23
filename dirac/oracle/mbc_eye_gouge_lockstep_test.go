package oracle

import "testing"

// Cabal Coffers ({2},{T}: Add {B} for each Swamp you control) under Realmwright's
// chosen Swamp is a Swamp and has the intrinsic "{T}: Add {B}" (CR 305.6). gorge
// skipped appending that ability because the printed one has Produced$ B, so it
// offered neither Eye Gouge, Fatal Push nor Lochmere Serpent's {B} ability to a
// player whose only land was the Coffers (decision 505 of tail-41-c000 pair 1).
func TestMBCEyeGougeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "costed_mana_ability_keeps_granted_basic_type_intrinsic")
}
