package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedReturnFindsNothingAfterGraveyardExile pins CR 400.7 for a delayed
// trigger's remembered object across a move that does not cross the
// battlefield boundary: The Guardian of Forever sacrifices a Bears and
// remembers the card in the graveyard; if something else exiles it from there
// before the end step it is a new object the delayed trigger cannot find. The
// registration's stamp carries the object's ZoneGen (it advances on every zone
// change), not only its Incarnation. The control arm has no extra move and
// returns the Bears.
func TestDelayedReturnFindsNothingAfterGraveyardExile(t *testing.T) {
	t.Parallel()
	for _, exile := range []bool{false, true} {
		e := handEngine(t)
		guardian := e.G.AddObject(corpusAlternativeCard(t, "The Guardian of Forever"), 0)
		guardian.Zone = state.ZBattlefield
		bears := e.G.AddObject(card(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
		bears.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{guardian.ID, bears.ID})
		e.G.Players[0].Pool[state.MC] = 4

		activateAbilityOn(t, e, guardian.ID)
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != bears.ID {
			t.Fatalf("exile=%v: sacrifice choice %+v, want the Bears", exile, d)
		}
		submitChoices(t, e, 0)
		passUntil(t, e, func() bool { return len(e.G.Stack) == 0 && len(e.G.Delayed) == 1 })
		if z := e.G.Obj(bears.ID).Zone; z != state.ZGraveyard {
			t.Fatalf("exile=%v: precondition: Bears in %v after the sacrifice, want the graveyard", exile, z)
		}
		stamps := e.G.Delayed[0].RememberedStamps
		if len(stamps) != 1 || stamps[0].Obj != bears.ID || stamps[0].Incarnation != e.G.Obj(bears.ID).Incarnation ||
			stamps[0].ZoneGen != e.G.Obj(bears.ID).ZoneGen {
			t.Fatalf("exile=%v: registration stamps %+v, want the Bears at its current Incarnation and ZoneGen", exile, stamps)
		}
		if exile {
			e.emit(events.Event{Kind: events.MoveZone, Obj: bears.ID, From: state.ZGraveyard, To: state.ZExile})
			o := e.G.Obj(bears.ID)
			if o.Zone != state.ZExile || stamps[0].Incarnation != o.Incarnation || stamps[0].ZoneGen == o.ZoneGen {
				t.Fatalf("precondition: Bears in %v, Incarnation %d vs stamp %d, ZoneGen %d vs stamp %d; want exile, equal Incarnation, differing ZoneGen",
					o.Zone, o.Incarnation, stamps[0].Incarnation, o.ZoneGen, stamps[0].ZoneGen)
			}
		}
		passUntil(t, e, func() bool {
			return len(e.G.Delayed) == 0 && len(e.G.Stack) == 0 && e.Pending() != nil
		})
		want := state.ZBattlefield
		if exile {
			want = state.ZExile
		}
		if z := e.G.Obj(bears.ID).Zone; z != want {
			t.Fatalf("exile=%v: Bears in %v at the end step, want %v", exile, z, want)
		}
	}
}
