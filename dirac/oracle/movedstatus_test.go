package oracle

import "testing"

// Desculpting Blast's "if it was attacking" and Brackish Blunder's "if it was
// tapped" read the bounced target's last battlefield status (CR 608.2h).
func TestMovedTargetLastStatusLockstep(t *testing.T) {
	synthPairs(t, "std-eoe", []int{7, 19, 34}, nil)
	synthPairs(t, "std-lci", []int{6, 14, 28}, nil)
}
