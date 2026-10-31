package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// grantedOnWatcher builds a lord granting keyword and a creature whose printed
// lines observe the targeting and attack events without ever matching them, so
// the creature reaches the walk's printed path and only its grant can fire.
func grantedOnWatcher(t *testing.T, keyword string) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t)
	onBoard(t, e, 0, "Name:Lord\nTypes:Artifact\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ "+keyword+" | Description$ Grant it.\nOracle:x\n")
	watcher := onBoard(t, e, 0, "Name:Watcher\nTypes:Creature Bear\nPT:2/2\n"+
		"T:Mode$ BecomesTarget | ValidTarget$ Land | Execute$ TrigGain | TriggerDescription$ Never matches.\n"+
		"T:Mode$ Attacks | ValidCard$ Land | Execute$ TrigGain | TriggerDescription$ Never matches.\n"+
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n")
	return e, watcher
}

// TestGrantedWardFiresBesideLivePrintedTriggers pins CR 702.21a: granted ward
// triggers on a permanent whose own printed lines also observe the event.
func TestGrantedWardFiresBesideLivePrintedTriggers(t *testing.T) {
	t.Parallel()
	e, watcher := grantedOnWatcher(t, "Ward:2")
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{watcher}})
	e.putTriggersOnStack()
	if stackAbilityForSource(e, watcher, "Ward") == 0 {
		t.Fatalf("no ward on the stack: %v", e.G.Stack)
	}
}

// TestGrantedDethroneFiresBesideLivePrintedTriggers pins CR 702.105a on the
// same path.
func TestGrantedDethroneFiresBesideLivePrintedTriggers(t *testing.T) {
	t.Parallel()
	e, watcher := grantedOnWatcher(t, "Dethrone")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{watcher}})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(watcher).Counter("P1P1"); got != 1 {
		t.Fatalf("%d counters, want 1", got)
	}
}
