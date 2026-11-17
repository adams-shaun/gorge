package oracle

import "testing"

// Skyclave Shade's landfall trigger leaves an effect whose static grants "you
// may cast it from your graveyard this turn" (CR 611.2a). The Shade is cast
// through that grant, then sacrificed back to the graveyard; gorge re-reads
// the still-active effect-delivered grant and offers the cast again, while
// the kernel's permission record was spent by the first cast and never
// re-armed (decision 655 of tail-32 pair 50: gorge offered cast(#66,-,3),
// the kernel offered only pass).
func TestMBCSkyclaveShadeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "skyclave_shade_mayplay_reentry")
}
