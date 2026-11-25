package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// highcliffFelidarETB puts Highcliff Felidar in seat 0's hand, moves it to the
// battlefield (the same MoveZone a resolved cast emits), and resolves its enter
// trigger. opponentCard is placed on seat 1's battlefield first when non-empty.
func highcliffFelidarETB(t *testing.T, opponentCard string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	var opponent state.ObjID
	wantOpponent := 0
	if opponentCard != "" {
		opponent = onBoard(t, e, 1, opponentCard)
		wantOpponent = 1
	}
	if got := len(e.G.Zone(state.ZBattlefield, 1)); got != wantOpponent {
		t.Fatalf("precondition: seat 1 battlefield has %d creatures, want %d", got, wantOpponent)
	}
	felidar := e.G.AddObject(corpusCard(t, "Highcliff Felidar"), 0)
	felidar.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), felidar.ID))
	id := felidar.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != id {
		t.Fatalf("precondition: stack = %v, want Highcliff Felidar's enter trigger", e.G.Stack)
	}
	// A trigger resolving with nothing to choose poses no priority decision,
	// so drive resolveTop and answer any ask it poses.
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil {
			if len(d.Options) == 0 {
				t.Fatalf("precondition: decision %+v offers nothing to choose", d)
			}
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		e.resolveTop()
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: stack still %v after draining the enter trigger", e.G.Stack)
	}
	return e, id, opponent
}

// This is the RepeatEach sibling of TestRememberWriterThatWroteNothingMovesNothing:
// with no opponent creature the RepeatEach body's ChooseCard has nothing to
// choose, so nothing is written to Remembered and DestroyAll's
// Creature.IsRemembered must name nothing -- not the enter trigger's fire-time
// capture, which is the Felidar itself.
func TestHighcliffFelidarNoOpponentCreatureDestroysNothing(t *testing.T) {
	t.Parallel()
	e, felidar, _ := highcliffFelidarETB(t, "")
	if o := e.G.Obj(felidar); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Highcliff Felidar zone = %v, want battlefield: no creature was chosen, so nothing is remembered", zoneName(e.G.Obj(felidar)))
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.DestroyProposal && ev.Obj == felidar {
			t.Fatalf("Highcliff Felidar was proposed for destruction at %+v; IsRemembered fell back to the ETB capture", ev)
		}
	}
}

// The sibling with an opponent creature: the RepeatEach body writes the pick,
// so it replaces the capture and DestroyAll kills that creature, not the
// Felidar.
func TestHighcliffFelidarOpponentCreatureIsDestroyed(t *testing.T) {
	t.Parallel()
	e, felidar, bear := highcliffFelidarETB(t, "Name:Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if o := e.G.Obj(felidar); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Highcliff Felidar zone = %v, want battlefield", zoneName(e.G.Obj(felidar)))
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("chosen opponent creature zone = %v, want graveyard", zoneName(e.G.Obj(bear)))
	}
}
