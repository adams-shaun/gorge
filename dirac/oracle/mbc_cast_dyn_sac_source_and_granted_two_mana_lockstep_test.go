package oracle

import "testing"

// CR 601.2g-h: Akroma, Angel of Wrath ({5}{W}{W}{W}) is a legal cast when the
// white comes from Gift of Paradise's two mana of one colour plus a petal of
// Lotus Blossom, and the rest from lands. Gorge offered the cast; the kernel
// did not (decision 744 of 744; tail-74-c000 pair 123, seed 746980).
func TestMBCAkromaAngelOfWrathReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_dyn_sac_source_and_granted_two_mana")
}
