package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/resolve"
	"github.com/adams-shaun/gorge/engine/state"
)

// A planned shuffle materializes the already-generated suffix into the chance
// prefix. UnplannedDraws must keep counting those earlier draws as unplanned,
// not let the longer prefix retroactively claim a planner supplied them.
func TestUnplannedDrawsSurvivesLaterPlannedShuffle(t *testing.T) {
	t.Parallel()
	cfg := chanceConfig(t)
	planned := false
	e, err := NewHypotheticalPlanned(cfg, nil, func(ctx ShuffleContext) ([]state.ObjID, error) {
		if !planned {
			return nil, nil
		}
		out := make([]state.ObjID, len(ctx.Library))
		for i, card := range ctx.Library {
			out[i] = card.ID
		}
		return out, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	base := e.UnplannedDraws()
	if base == 0 {
		t.Fatal("genesis recorded no unplanned draws")
	}
	e.Rand(2)
	if got := e.UnplannedDraws(); got != base+1 {
		t.Fatalf("after an unplanned draw: %d, want %d", got, base+1)
	}
	planned = true
	e.ShuffleLibrary(0, e.G.Zone(state.ZLibrary, 0))
	if got := e.UnplannedDraws(); got != base+1 {
		t.Fatalf("planned shuffle reclassified an unplanned draw: %d, want %d", got, base+1)
	}

	cp := asResolve(e).Checkpoint()
	evEnd := len(e.L.Events)
	e.Rand(3)
	if got := e.UnplannedDraws(); got != base+2 {
		t.Fatalf("after a second unplanned draw: %d, want %d", got, base+2)
	}
	asResolve(e).Restore(&resolve.Checkpoint{S0: cp}, evEnd)
	if got := e.UnplannedDraws(); got != base+1 {
		t.Fatalf("restore did not rewind the unplanned count: %d, want %d", got, base+1)
	}
}
