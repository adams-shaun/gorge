package oracle

import "testing"

// Level up (CR 702.87) is a sorcery-speed activated ability of the card.
// tail-45 pair 7, decision 899: only the kernel offered Ikiral Outrider's
// Level up at p0's priority.
func TestMBCIkiralOutriderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "level_up_offer_ikiral_outrider")
}
