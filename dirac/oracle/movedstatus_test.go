package oracle

import "testing"

// Desculpting Blast's "if it was attacking" and Brackish Blunder's "if it was
// tapped" read the bounced target's last battlefield status (CR 608.2h).
func TestMovedTargetLastStatusLockstep(t *testing.T) {
	pinnedGames(t, "std-eoe", Config{GorgeAutopaySurface: true}, cardOut("Desculpting Blast"))
	pinnedGames(t, "std-lci", Config{GorgeAutopaySurface: true}, cardOut("Brackish Blunder"))
}

// A token copy bounced to hand is read as it last was on the battlefield, not as a
// ceased copy that matches nothing (CR 608.2h, 707.10h): Brackish Blunder, found by Hurl
// into History's discover, bounces a tapped token copy and makes its Map.
func TestMovedTokenCopyLastStatusLockstep(t *testing.T) {
	pinnedGames(t, "std-lci", Config{GorgeAutopaySurface: true}, cardOut("Brackish Blunder"), cardOut("Hurl into History"))
}
