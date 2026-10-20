package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/resolve"
	"github.com/adams-shaun/gorge/engine/state"
)

// The resolution kernel's checkpoint/restore pair belongs to the same engine
// lineage, so a restore must keep the planner and resume its shuffle ordinal.
// Clone drops both by design (a search world must not inherit the live game's
// planner); the checkpoint carries them back.
func TestCheckpointRestoreKeepsHypotheticalPlanner(t *testing.T) {
	t.Parallel()
	cfg := chanceConfig(t)
	var calls int
	var last ShuffleContext
	e, err := NewHypotheticalPlanned(cfg, []ChanceDraw{{Bound: 2, Value: 0}}, func(ctx ShuffleContext) ([]state.ObjID, error) {
		calls++
		last = ctx
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
	cp := asResolve(e).Checkpoint()
	evEnd := len(e.L.Events)
	before := calls
	e.ShuffleLibrary(0, e.G.Zone(state.ZLibrary, 0))
	if calls != before+1 || last.Ordinal != 1 {
		t.Fatalf("planned shuffle before restore: calls=%d ordinal=%d", calls, last.Ordinal)
	}
	asResolve(e).Restore(&resolve.Checkpoint{S0: cp}, evEnd)
	before = calls
	e.ShuffleLibrary(0, e.G.Zone(state.ZLibrary, 0))
	if calls != before+1 {
		t.Fatal("restore dropped the hypothetical planner")
	}
	if last.Ordinal != 1 {
		t.Fatalf("restore did not rewind the shuffle ordinal: %d", last.Ordinal)
	}
}
