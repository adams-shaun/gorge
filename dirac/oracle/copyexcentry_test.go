package oracle

import "testing"

// A token copy enters with its exceptions, so the entry's replacements read them:
// Saheeli's artifact copy enters tapped under an opposing Dauntless Dismantler
// (CR 707.9, 614.12).
func TestCopyExceptionsBeforeEntryReplacements(t *testing.T) {
	synthPairs(t, "std-lci", []int{18, 320, 490}, nil)
}
