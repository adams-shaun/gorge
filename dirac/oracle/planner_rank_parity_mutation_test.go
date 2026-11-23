package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestPlannerRankParityMutation replays both planner-rank-parity recordings
// with dirac.DebugPlannerFlexCensus set (the pre-fix behaviour: the plain-play
// walk prices with payDescNow instead of the play's own payment descriptor,
// and an Improvise cast whose announcements-beside-a-credit do not lower the
// composed price falls back to the manual window) and requires each recorded
// divergence back, so the parity fix is pinned to its mechanism: the same
// recorded game, replayed with the flag, must split again.
func TestPlannerRankParityMutation(t *testing.T) {
	// Sequential: the flag is a package global both subtests read.
	for _, name := range []string{"planner_rank_parity", "planner_rank_parity_b"} {
		t.Run(name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			old := dirac.DebugPlannerFlexCensus
			dirac.DebugPlannerFlexCensus = true
			defer func() { dirac.DebugPlannerFlexCensus = old }()
			res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
			if res.Div == nil {
				t.Fatalf("%s: the mutation did not diverge (%d actions served)", name, len(r.Actions))
			}
			t.Logf("%s: diverged at %d (%s %s) under the mutation", name, res.Div.At, res.Div.What, res.Div.Field)
		})
	}
}
