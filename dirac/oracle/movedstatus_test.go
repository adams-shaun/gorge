package oracle

import "testing"

// Desculpting Blast's "if it was attacking" and Brackish Blunder's "if it was
// tapped" read the bounced target's last battlefield status (CR 608.2h).
func TestMovedTargetLastStatusLockstep(t *testing.T) {
	synthPairsReaching(t, "std-eoe", Config{GorgeAutopaySurface: true}, []int{19, 34, 3}, cardOut("Desculpting Blast"))
	synthPairsReaching(t, "std-lci", Config{GorgeAutopaySurface: true}, []int{3, 11, 24}, cardOut("Brackish Blunder"))
}

// A token copy bounced to hand is read as it last was on the battlefield, not as a
// ceased copy that matches nothing (CR 608.2h, 707.10h): Brackish Blunder, found by Hurl
// into History's discover, bounces a tapped token copy and makes its Map.
func TestMovedTokenCopyLastStatusLockstep(t *testing.T) {
	synthPairsReaching(t, "std-lci", Config{GorgeAutopaySurface: true}, []int{11, 248}, cardOut("Brackish Blunder"), cardOut("Hurl into History"))
}
