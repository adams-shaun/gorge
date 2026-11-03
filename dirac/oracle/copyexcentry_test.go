package oracle

import "testing"

// A token copy enters with its exceptions, so the entry's replacements read them:
// Saheeli's artifact copy enters tapped under an opposing Dauntless Dismantler
// (CR 707.9, 614.12).
func TestCopyExceptionsBeforeEntryReplacements(t *testing.T) {
	synthPairsReaching(t, "std-lci", Config{GorgeAutopaySurface: true}, []int{22, 26, 35}, onBattlefield("Dauntless Dismantler"))
}
