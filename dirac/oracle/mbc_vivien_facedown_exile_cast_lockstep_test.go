package oracle

import "testing"

// Vivien, Champion of the Wilds's -2 exiles one of the top three cards face
// down; for as long as it stays exiled its controller may look at it and may
// cast it if it's a creature card. Here the exiled card is Aesthir Glider,
// and gorge offers cast(#40{Aesthir Glider},-,3) from exile at the next
// priority (decision 247) while dirac did not. The evidence loop filed the
// class on Grappler Spider (tail-20-c000 pair 6 at land 94197a4cea); the
// recording plays the same shape on its own decks' table.
func TestMBCGrapplerSpiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vivien_facedown_exile_cast")
}
