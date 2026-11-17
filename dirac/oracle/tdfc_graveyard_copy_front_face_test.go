package oracle

import "testing"

// TestTdfcGraveyardCopyFrontFace replays a game where Feldon of the Third Path
// copies a daybound double-faced creature card from its graveyard at night. The
// token is double-faced (CR 707.8a) and a daybound permanent is back face up at
// night (CR 702.145b/c), so it is the back face's 3/4 in both engines.
func TestTdfcGraveyardCopyFrontFace(t *testing.T) {
	synthReproLockstep(t, "repro/tdfc_graveyard_copy_front_face", "tail-56", Config{GorgeAutopaySurface: true})
}
