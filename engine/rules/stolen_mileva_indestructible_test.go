package rules

// A static a permanent gained follows the permanent when its controller
// changes: "other creatures you control" is read against the CURRENT
// controller (CR 109.5, 611.3a). Before the fix only may-play land grants
// re-read their source's controller, so a stolen Mileva kept granting
// indestructible to the old controller's creatures.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestStolenMilevaIndestructibleFollowsController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := edrBoard(t, reg, 11, map[string]state.Zone{
		"Tenth District Hero": state.ZBattlefield,
		"Grizzly Bears":       state.ZBattlefield,
		"Hill Giant":          state.ZGraveyard,
		"Serra Angel":         state.ZGraveyard,
	})
	hero, bears := ids["Tenth District Hero"], ids["Grizzly Bears"]
	if e.G.Obj(hero).Zone != state.ZBattlefield || e.G.Obj(bears).Zone != state.ZBattlefield {
		t.Fatalf("precondition: hero/bears not on the battlefield the static reads")
	}
	activate := func(idx int, mana string, evidence state.ObjID) {
		t.Helper()
		addMana(t, e, 0, mana)
		animGrantSeatZero(t, e)
		submitChoices(t, e, abilityOption(t, e, hero, idx).Index)
		milevaEvidence(t, e, evidence)
		passUntilStackEmpty(t, e, 20)
	}
	activate(0, "CW", ids["Hill Giant"])
	if !slices.Contains(e.Derived(hero).Types, "Detective") {
		t.Fatalf("precondition: Tenth District Hero did not become a Detective")
	}
	activate(1, "CCW", ids["Serra Angel"])
	if got := e.Name(hero); got != "Mileva, the Stalwart" {
		t.Fatalf("precondition: the hero did not become Mileva (name %q)", got)
	}
	if e.G.Obj(hero).Controller != 0 || e.G.Obj(bears).Controller != 0 {
		t.Fatalf("precondition: hero/bears are not both seat 0's (hero %d, bears %d)",
			e.G.Obj(hero).Controller, e.G.Obj(bears).Controller)
	}
	if !e.HasKeyword(bears, "Indestructible") {
		t.Fatalf("Mileva's other creature is not indestructible")
	}

	// Steal Mileva. Seat 0 no longer controls the source static, so its
	// "other creatures you control" no longer reaches seat 0's bears.
	e.emit(events.Event{Kind: events.ControlChange, Obj: hero, Player: 1})
	if got := e.G.Obj(hero).Controller; got != 1 {
		t.Fatalf("precondition: Mileva did not change controller (controller %d)", got)
	}
	if e.HasKeyword(bears, "Indestructible") {
		t.Fatalf("the stolen Mileva still grants indestructible to its old controller's creature")
	}

	// Hand the bears to the thief: now they ARE another creature the
	// current controller controls, so the same static reaches them.
	e.emit(events.Event{Kind: events.ControlChange, Obj: bears, Player: 1})
	if got := e.G.Obj(bears).Controller; got != 1 {
		t.Fatalf("precondition: the bears did not change controller (controller %d)", got)
	}
	if !e.HasKeyword(bears, "Indestructible") {
		t.Fatalf("the stolen Mileva does not grant indestructible to its current controller's creature")
	}
	replayCheck(t, e, cfg)
}
