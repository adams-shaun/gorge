package oracle

import "testing"

// Azor's Elocutors ({3}{W/U}{W/U}) is castable from six untapped lands after
// Archway of Innovation granted the turn's next spell Improvise (CR 702.126a:
// improvise is optional, so the grant never removes a spell's plain payment).
// Both engines must offer the cast at decision 681 (tail-15-c000 pair 56).
func TestMBCAzorSElocutorsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_improvise_hybrid_cast")
}
