package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Quirion Explorer ("{T}: Add one mana of any color that a land an opponent
// controls could produce") wears the opponent's Spirit Shackle, whose
// "whenever enchanted creature becomes tapped" counter is an unvetted Taps
// watcher. gorge's planner never consults it for a reflected ability (its
// interference walk runs per production, and AB$ ManaReflected has none), so
// the Explorer stays an ordinary census unit and Dragon-Scarred Bear's {2}{G}
// is offered from an untapped Plains, Forest and the Explorer (CR 601.2g,
// 605.3a). The kernel deferred the Explorer as a Taps-deferred gap and, on
// the autopay surface, left the cast out of the priority offer (tail-75-c002
// pair 5, decision 313).
func TestMBCDragonScarredBearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_reflected_source")
}

// TestMBCDragonScarredBearReprosLockstepMutation: with the reflected source
// classed by the Taps watchers again the recording diverges, proof it
// exercises dirac.reflTapsUnwatched.
func TestMBCDragonScarredBearReprosLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "taps_deferred_reflected_source.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugNoReflTapsUnwatched = true
	defer func() { dirac.DebugNoReflTapsUnwatched = false }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatal("the recording replayed clean with the reflected source deferred: it does not exercise reflTapsUnwatched")
	}
}
