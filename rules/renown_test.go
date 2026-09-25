package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func renownMarks(e *Engine, id state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.AlterAttribute && ev.Text == "Renowned" && ev.Obj == id {
			out = append(out, ev)
		}
	}
	return out
}

func TestKnightOfThePilgrimsRoadRenownMarksOnCombatDamage(t *testing.T) {
	e, _ := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Knight of the Pilgrim's Road"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	id := findBattlefield(t, e, 0, "Knight of the Pilgrim's Road", 0)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Renowned || o.Counter("P1P1") != 0 {
		t.Fatalf("precondition: battlefield, unrenowned 0/0-counter setup not met: %+v", o)
	}
	e.askAttackers()
	submitAttackers(t, e, id)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
		t.Fatalf("Renown 1 yielded %d +1/+1 counters, want 1", got)
	}
	if !e.G.Obj(id).Renowned {
		t.Fatal("the Knight did not become renowned")
	}
	marks := renownMarks(e, id)
	if len(marks) != 1 || marks[0].Amount != 1 {
		t.Fatalf("want one Renowned mark with Amount 1, got %+v", marks)
	}
}

func TestConstableOfTheRealmRenownPutsTwoCountersOnce(t *testing.T) {
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Constable of the Realm"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	id := findBattlefield(t, e, 0, "Constable of the Realm", 0)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Renowned || o.Counter("P1P1") != 0 {
		t.Fatalf("precondition: battlefield, unrenowned 0-counter setup not met: %+v", o)
	}
	e.askAttackers()
	submitAttackers(t, e, id)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("Renown 2 yielded %d +1/+1 counters, want 2", got)
	}
	if !e.G.Obj(id).Renowned {
		t.Fatal("Constable did not become renowned")
	}
	if marks := renownMarks(e, id); len(marks) != 1 {
		t.Fatalf("want exactly one Renowned mark, got %+v", marks)
	}
	// Exercise a second, separately emitted combat-damage trigger in the
	// same turn. The trigger may resolve, but its Renown$ effect's intervening
	// condition must prevent both another counter batch and another mark.
	e.damaging = id
	e.combatDamaging = true
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 2})
	e.combatDamaging = false
	e.damaging = 0
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(id).Counter("P1P1"); got != 2 {
		t.Fatalf("second combat damage changed Renown counters to %d, want still 2", got)
	}
	if marks := renownMarks(e, id); len(marks) != 1 {
		t.Fatalf("second combat damage emitted another Renowned mark: %+v", marks)
	}
	// A logged departure ends the designation; a subsequent permanent entry
	// is a different game object under CR 400.7.
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZHand})
	if e.G.Obj(id).Renowned {
		t.Fatal("leaving the battlefield did not clear Renowned")
	}
	replayCheck(t, e, cfg)
}

func TestEnshroudingMistConditionPresentReadsRenowned(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name     string
		renowned bool
		wantTap  bool
	}{
		{name: "not renowned remains tapped", wantTap: true},
		{name: "renowned untaps", renowned: true, wantTap: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, ids := targetingBoard(t, reg, []string{"Goblin Glory Chaser"}, []string{"Enshrouding Mist"})
			target, mist := ids["Goblin Glory Chaser"], ids["Enshrouding Mist"]
			if e.G.Obj(target).Zone != state.ZBattlefield || e.G.Obj(mist).Zone != state.ZHand {
				t.Fatal("precondition: target must be on battlefield and Mist in hand")
			}
			e.emit(events.Event{Kind: events.Tap, Obj: target})
			if tc.renowned {
				e.emit(events.Event{Kind: events.AlterAttribute, Obj: target, Text: "Renowned", Amount: 1})
			}
			if !e.G.Obj(target).Tapped || e.G.Obj(target).Renowned != tc.renowned {
				t.Fatalf("precondition: tapped=%v renowned=%v", e.G.Obj(target).Tapped, e.G.Obj(target).Renowned)
			}
			addMana(t, e, 1, "W")
			castStrikeAt(t, e, mist, target, 1)
			passUntilResolved(t, e, 30)
			if got := e.G.Obj(target).Tapped; got != tc.wantTap {
				t.Fatalf("after Enshrouding Mist, tapped=%v, want %v (renowned=%v)", got, tc.wantTap, tc.renowned)
			}
		})
	}
}

func TestGoblinGloryChaserRenownedGainsMenace(t *testing.T) {
	e, _ := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Goblin Glory Chaser"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	id := findBattlefield(t, e, 0, "Goblin Glory Chaser", 0)
	if e.G.Obj(id).Zone != state.ZBattlefield || e.G.Obj(id).Renowned || e.HasKeyword(id, "Menace") {
		t.Fatalf("precondition: Glory Chaser should be unrenowned on battlefield without menace")
	}
	e.askAttackers()
	submitAttackers(t, e, id)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	if !e.G.Obj(id).Renowned || !e.HasKeyword(id, "Menace") || !slices.Contains(e.Keywords(id), "Menace") {
		t.Fatalf("renowned Glory Chaser did not gain menace: renowned=%v keywords=%v", e.G.Obj(id).Renowned, e.Keywords(id))
	}
}
