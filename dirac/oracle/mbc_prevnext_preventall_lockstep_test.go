package oracle

import "testing"

// CR 616.1 with CR 615.7: D'Avenant Healer's depleting "prevent the next 1
// damage" shield sits on Cho-Manno, Revolutionary beside the creature's own
// printed "Prevent all damage that would be dealt to CARDNAME." Both prevent
// the noncombat event whole, but the order decides whether the shield is
// spent, so gorge poses the order ask and the kernel must pose it too, not
// stall at prevnext (tail-40-c000 pair 74, decision 1255).
func TestMBCTail40ReprosPreventallLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevnext_preventall")
}
