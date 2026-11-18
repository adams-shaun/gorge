package oracle

import "testing"

// CR 118.9: a card exiled with Dauthi Voidwalker's free permission is also
// the subject of Dragon's Smile's {0} alternative. Gorge lists the free plain
// cast (AltCostIndex 0) and the {0} alternative (AltCostIndex 1) beside each
// other; dirac offered both as one action and stalled (zeroaltshared).
func TestMBCTail15ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "zero_alt_beside_free_permission")
}
