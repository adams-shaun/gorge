package oracle

import "testing"

// Desculpting Blast's "if it was attacking" and Brackish Blunder's "if it was
// tapped" read the bounced target's last battlefield status (CR 608.2h).
func TestMovedTargetLastStatusLockstep(t *testing.T) {
	synthPairs(t, "std-eoe", []int{7, 19, 34}, nil)
	synthPairs(t, "std-lci", []int{6, 14, 28}, nil)
}

// A token copy bounced to hand is read as it last was on the battlefield, not as a
// ceased copy that matches nothing (CR 608.2h, 707.10h): Brackish Blunder, found by Hurl
// into History's discover, bounces a tapped token copy and makes its Map.
func TestMovedTokenCopyLastStatusLockstep(t *testing.T) {
	synthPairs(t, "std-lci", []int{3411}, nil)
}
