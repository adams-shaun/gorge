package oracle

import "testing"

const diregrafFlashbackRepro = "diregraf_rebirth_flashback_two_filter_budget"

// Diregraf Rebirth's flashback cast {5}{B}{G} (pool tail-51, surface autopay,
// the recorded shrunk game; decision 1183): the board holds four Swamps,
// Artifact Unknown Shores and Heap Gate ({1},{T}: add one mana of any color)
// beside Turbulent Shore and an Eldrazi Spawn. gorge's prefix search walks its
// 160-node budget (potentialScriptNodes) across the two filter lands, dies
// before a script for the flashback cast is found and withholds the cast as
// census_incomplete, offering only pass and the land play. The kernel used to
// price it: #2392's graveyard-cast pip hint led its grant and tap-another
// searches. Since the hint is read only by their second runs, after gorge's
// printed-need order finds nothing (hotfix/oracle-hermitic), both engines
// withhold the flashback cast, the card is no longer refused
// (dirac/support/unrefused.tsv), and the recording replays in lockstep.
func TestMBCDiregrafRebirthReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, diregrafFlashbackRepro)
}
