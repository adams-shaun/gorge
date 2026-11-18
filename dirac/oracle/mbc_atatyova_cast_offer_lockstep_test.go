package oracle

import "testing"

// A-Tatyova, Steward of Tides ({G}{G}{U}) in hand beside Lotus Blossom and
// lands: gorge offers the cast at decision 123 (pool tail-74, mulligans 1,
// surface full); the kernel offered the land plays and pass only.
func TestMBCATatyovaStewardOfTidesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "atatyova_cast_offer")
}
