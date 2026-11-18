package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const theMightyThorJaneFosterReturnTappedRepro = "the_mighty_thor_jane_foster_return_tapped"

// The Mighty Thor, Jane Foster's attack trigger exiles up to one target
// nontoken artifact or creature, then returns that card to the battlefield
// tapped (CR 110.5's entry state). The recorded game (std-msh pair 317) returns
// A.I.M. Synthoids and must hold the Tapped flag on both sides at decision 691.
func TestMBCTheMightyThorJaneFosterReturnTappedLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro(filepath.Join("testdata", "repro", theMightyThorJaneFosterReturnTappedRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), cardset.Raw(printedDelveSet(t, r.Cards)), r, pol)
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("recording served %d of %d actions (%d skipped): its game path has drifted", pol.next, len(r.Actions), pol.Skipped)
	}
	if !res.Clean() {
		t.Fatalf("The Mighty Thor's recorded game no longer replays clean: div %+v, stall %q", res.Div, res.Stall)
	}
}
