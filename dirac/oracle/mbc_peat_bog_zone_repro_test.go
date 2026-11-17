package oracle

import "testing"

// CR 608.2g: paying an unless / ward cost may activate mana abilities first,
// and Peat Bog's "{T}, Remove a depletion counter from Peat Bog: Add {B}{B}
// ... If there are no depletion counters on Peat Bog, sacrifice it" is one.
// tail-55 pair 45 (Acorn Catapult focus, autopay) paid the window with Peat
// Bog's second activation: gorge removed the last counter and sacrificed it,
// while the kernel left it on the battlefield with one counter. The
// recording pins the fix in "dirac: an unless-pay window counts a fixed
// depletion-fee source (CR 608.2g)".
func TestMBCPeatBogZoneReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_consumes_depletion_source")
}
