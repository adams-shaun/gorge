package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGoadedDiesTriggerLooksBack verifies Baeloth Barrityl, Entertainer's
// "Whenever a goaded attacking or blocking creature dies, you create a
// Treasure token" trigger fires through the REAL engine path when an
// event-backed (CR 701.38) goaded attacker dies to combat damage. CR 603.10a:
// a leaves-the-battlefield trigger looks back in time, so the goad the
// creature held just before it died counts -- pruneGoads' clear on the zone
// move must not be what the matcher reads.
func TestGoadedDiesTriggerLooksBack(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t)
	baelothCard := corpusAlternativeCard(t, "Baeloth Barrityl, Entertainer")
	baeloth := onBoardCard(t, e, 0, baelothCard)
	// PT:9/9 keeps the victim outside Baeloth's own Goad$ True static
	// (Creature.powerLTY+OppCtrl, threshold his power 5): the test isolates
	// the event-backed relationship (CR 701.38) from the static route.
	victim := onBoardReady(t, e, 1, "Name:Victim\nTypes:Creature\nPT:9/9\nOracle:x\n")
	v := e.G.Obj(victim)
	v.IsAttacking = true
	v.Attacking = 0

	// Preconditions: the victim is an attacking creature whose only goad is
	// the event-backed relationship, not Baeloth's own Goad$ True static.
	if e.G.Obj(baeloth).Zone != state.ZBattlefield || v.Zone != state.ZBattlefield || !v.IsAttacking {
		t.Fatal("precondition: Baeloth and its attacking victim must be on the battlefield")
	}
	if e.goadedBy(v, 0) {
		t.Fatal("precondition: victim is statically goaded too; the test must isolate the event-backed relationship")
	}
	e.emit(events.Event{Kind: events.Goad, Obj: victim, Player: 0})
	if len(v.Goads) != 1 {
		t.Fatalf("precondition: victim's event-backed goads=%v, want exactly one", v.Goads)
	}

	var diesTrigger *cards.Trigger
	for i := range e.G.Obj(baeloth).Face().Triggers {
		tr := &e.G.Obj(baeloth).Face().Triggers[i]
		if tr.Mode == "ChangesZone" && strings.Contains(tr.Params["ValidCard"], "IsGoaded") {
			diesTrigger = tr
			break
		}
	}
	if diesTrigger == nil {
		t.Fatal("precondition: Baeloth's corpus ChangesZone trigger with IsGoaded was not parsed")
	}

	before := len(e.pendingTriggers)
	e.emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZBattlefield,
		To: state.ZGraveyard, Player: v.Controller})
	if live := e.G.Obj(victim); live == nil || live.Zone != state.ZGraveyard {
		t.Fatalf("precondition: victim's current object is not in the graveyard: %+v", live)
	}
	// The zone move itself must not have been what fired anything else.
	for _, pt := range e.pendingTriggers[before:] {
		if pt.Source == baeloth {
			t.Logf("Baeloth's trigger queued: SA=%v", pt.SA != nil)
			return
		}
	}
	t.Fatalf("Baeloth's goaded-attacker dies trigger was not queued on the death (pendingTriggers before=%d after=%d)", before, len(e.pendingTriggers))
}
