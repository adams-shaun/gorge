package oracle

import "testing"

// tail-42-c000 pair 49 (seed 283068, autopay): at p1's priority decision 269
// dirac offered a kicked cast of Caligo Skin-Witch (cast(#44,-,1)) alongside
// the plain one while gorge offered only the plain cast.
func TestMBCCaligoSkinWitchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "caligo_skin_witch_kicked_offer")
}
