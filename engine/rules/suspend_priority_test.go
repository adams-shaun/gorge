package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 117.3b: the final-counter trigger's resolution ends with the active
// player receiving priority. The free cast made through the may-cast ask
// returns it to them, not to the seat that held it in the previous step
// (seat 0 passed last in this fixture).
func TestSuspendCastGivesTheActivePlayerPriority(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Filler\nManaCost:3 R\nTypes:Sorcery\nOracle:x\n"))
	o := e.G.AddObject(card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:1:R\nOracle:x\n"), 1)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 1, []state.ObjID{o.ID})
	e.G.Players[1].Pool[state.MR] = 1
	e.beginCast(1, decision.Option{Kind: "cast", Obj: o.ID, Mode: "suspend"})
	e.beginTurn(1)
	if d := e.Pending(); d == nil || e.G.Active != 1 || len(d.Options) != 2 || d.Options[0].Kind != "suspend_cast_yes" {
		t.Fatalf("precondition: seat 1's upkeep does not offer the cast: active=%d %+v", e.G.Active, d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(o.ID).Zone != state.ZStack {
		t.Fatalf("precondition: the suspended creature was not cast: %s", e.G.Obj(o.ID).Zone)
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 1 {
		t.Fatalf("priority after the suspend cast = %+v, want the active player 1", d)
	}
}
