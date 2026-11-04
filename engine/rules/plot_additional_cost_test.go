package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlotCastPaysTheSpellsAdditionalSacrifice pins CR 701.34d / 118.9: the
// plotted free cast waives the mana cost only, so a spell whose own Cost$ is
// "sacrifice a creature" is offered only with a creature to sacrifice, and
// the cast asks for the sacrifice instead of putting the spell on the stack
// for nothing.
func TestPlotCastPaysTheSpellsAdditionalSacrifice(t *testing.T) {
	t.Parallel()
	spell := card(t, "Name:Sac Draw\nManaCost:B\nTypes:Instant\nA:SP$ Draw | Cost$ B Sac<1/Creature> | NumCards$ 2\nOracle:x\n")
	e := handEngine(t, spell)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZExile})
	e.G.Obj(id).PlottedTurn = 1
	e.G.Turn = 2
	e.G.Active, e.G.Priority = 0, 0
	e.G.Step = state.StepMain1
	if o := e.G.Obj(id); o.Zone != state.ZExile || o.PlottedTurn == 0 {
		t.Fatalf("precondition: plotted card must sit in exile: zone=%s plotted=%d", o.Zone, o.PlottedTurn)
	}
	// No creature to sacrifice: the free cast cannot be paid, so it is withheld.
	if findMode(e.legalActions(0), "plot_cast") >= 0 {
		t.Fatalf("plot_cast was offered with no creature to sacrifice: %+v", e.legalActions(0))
	}
	bear := card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := e.G.AddObject(bear, 0)
	b.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{b.ID})
	if findMode(e.legalActions(0), "plot_cast") < 0 {
		t.Fatalf("precondition: plot_cast must be offered with a creature to sacrifice: %+v", e.legalActions(0))
	}
	castMode(t, e, id, "plot_cast")
	d := e.Pending()
	if d == nil || d.Kind == decision.KPriority || len(d.Options) == 0 || d.Options[0].Kind != "sacrifice" {
		t.Fatalf("the plotted cast did not ask for its sacrifice: %+v", d)
	}
	submitChoices(t, e, 0)
	if got := e.G.Obj(b.ID).Zone; got != state.ZGraveyard {
		t.Fatalf("the sacrificed creature is in %s, want graveyard", got)
	}
	if got := e.G.Obj(id).Zone; got != state.ZStack {
		t.Fatalf("the plotted spell is in %s, want stack", got)
	}
}
