package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMadnessCastPriorityToActive: CR 117.3b -- after the madness trigger
// resolves with the spell cast, the active player receives priority with a
// fresh pass count; the opponent's earlier pass (which let the trigger
// resolve) does not carry over, so Fiery Temper resolves only after both
// players pass again.
func TestMadnessCastPriorityToActive(t *testing.T) {
	t.Parallel()
	e, cfg, d, temper := madnessAskAfterDiscard(t, 940, "Fiery Temper", nil, []string{"Mountain", "Mountain"})
	if e.G.Active != 0 {
		t.Fatalf("precondition: active player = %d, want 0", e.G.Active)
	}
	submitChoices(t, e, madnessOption(d, "yes"))
	for i := 0; i < 8; i++ {
		p := e.Pending()
		if p == nil || p.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, 0)
	}
	if z := e.G.Obj(temper).Zone; z != state.ZStack {
		t.Fatalf("precondition: Fiery Temper in %s, want stack (pending %+v)", z, e.Pending())
	}
	p := e.Pending()
	if p == nil || p.Kind != decision.KPriority || p.Player != 0 {
		t.Fatalf("after the madness cast pending = %+v, want priority for P0", p)
	}
	if e.G.Priority != 0 || e.G.Passes != 0 {
		t.Fatalf("priority holder %d passes %d, want holder 0 with a fresh count", e.G.Priority, e.G.Passes)
	}
	passOnceP(t, e)
	if z := e.G.Obj(temper).Zone; z != state.ZStack {
		t.Fatalf("Fiery Temper in %s after one pass, want still on the stack", z)
	}
	if p := e.Pending(); p == nil || p.Kind != decision.KPriority || p.Player != 1 {
		t.Fatalf("after P0 passes pending = %+v, want priority for P1", p)
	}
	passOnceP(t, e)
	if z := e.G.Obj(temper).Zone; z == state.ZStack {
		t.Fatalf("Fiery Temper still on the stack after both players passed")
	}
	replayCheck(t, e, cfg)
}
