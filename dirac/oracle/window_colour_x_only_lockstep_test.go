package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestWindowColourXOnlyReproLockstep replays an autopay-surface game whose
// planned cast (Vivisection) is paid from a Combo land that itself gains a
// basic land type: Luxury Suite under Urborg, Tomb of Yawgmoth. Gorge's
// witness search ranks the granted Swamp ability's plan over the printed
// Combo's ("intrinsic" sorts before "printed" in the final tie-break), so it
// floats {B}; the kernel's planner kept its own Produces preference ({R}),
// which the projection saw only in the pool. (The ticket's X-cost window was
// the surface the divergence was hunted on; the staged one-pair repro reaches
// the same witness-paid Combo colour through Vivisection, whose {3}{U} the
// X-shaped planner likewise declines.)
func TestWindowColourXOnlyReproLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "window_colour_x_only")
}

// TestWindowColourXOnlyMutation restores the planner's Produces preference
// for the witness-paid plan: the recorded game must diverge again, so the
// rule the repro pins is the colour the plan's taps take.
func TestWindowColourXOnlyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugWindowColourPlanner
	dirac.DebugWindowColourPlanner = true
	defer func() { dirac.DebugWindowColourPlanner = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "window_colour_x_only.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("mutation: no divergence in %d decisions", res.Decisions)
	}
	t.Logf("mutation: divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
