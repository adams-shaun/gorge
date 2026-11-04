package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTapTargetReadsTheShapeCensus: with the shape census built, the tap gate
// aims only at a planned cast or a cast the planner declines for its cost's
// shape; a castable card with neither is one no hand tap casts. Without the
// census the reading is the potential census alone.
func TestTapTargetReadsTheShapeCensus(t *testing.T) {
	const planned, shape, other state.ObjID = 11, 12, 13
	d := &decision.Decision{
		PotentialCastsKnown: true,
		PotentialCasts:      []state.ObjID{planned, shape, other},
		PaymentActions:      []decision.PaymentAction{{Cast: decision.PlannedCast{Object: planned}, Plans: []decision.PaymentPlan{{}}}},
	}
	for _, id := range []state.ObjID{planned, shape, other} {
		if !tapTarget(d, id) {
			t.Errorf("no shape census: obj %d is not a tap target", id)
		}
	}
	d.ShapeCastsKnown, d.PotentialShapeCasts = true, []state.ObjID{shape}
	for id, want := range map[state.ObjID]bool{planned: true, shape: true, other: false} {
		if got := tapTarget(d, id); got != want {
			t.Errorf("shape census: tapTarget(obj %d) = %v, want %v", id, got, want)
		}
	}
}
