package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Skyclave Apparition class (CR 400.7 pending referents): a leave trigger
// whose body returns the card its source's battlefield stint exiled
// (`Defined$ Remembered | ConditionPresent$ Card.ExiledWithSource`). The
// source dies, its leave trigger queues, and BEFORE the trigger resolves the
// source re-enters the battlefield from the graveyard (removal resolving,
// then a reanimation effect on top of it). Two things must hold at once:
//
//   - the re-entry is a NEW object (CR 400.7), so the entry fold clears its
//     remembered list -- the fix this file pins from the Curator side;
//   - the pending leave trigger belongs to the OLD object, so its body still
//     resolves the exiled card OUT of exile. The queue-time
//     TriggerContext.SourceRemembered snapshot is what answers the body's
//     `Defined$ Remembered` read after the entry clear.
//
// Without the snapshot the body reads the live (cleared) list and the exiled
// card is stranded in exile forever; without the entry clear the re-entered
// object answers a later `Card.IsRemembered` read with a former incarnation's
// list (the Curator of Destinies bug this file's sibling pins).
//
// Fixtures are inline (the licensing rule); the scripts mirror the corpus
// cards' own SVar shapes.

func ltbReentryEngine(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	exiler := card(t, "Name:Fixture Apparition\nManaCost:1 W\nTypes:Creature Spirit\nPT:2/2\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile\n"+
		"SVar:TrigExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature.OppCtrl | TgtPrompt$ Select target creature an opponent controls | RememberChanged$ True\n"+
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ OnReturn | TriggerDescription$ When CARDNAME leaves the battlefield, return the exiled card to the battlefield.\n"+
		"SVar:OnReturn:DB$ ChangeZone | Origin$ Exile | Destination$ Battlefield | Defined$ Remembered | ConditionDefined$ Remembered | ConditionPresent$ Card.ExiledWithSource\n"+
		"Oracle:x\n")
	removal := card(t, "Name:Vindicate\nManaCost:1 B W\nTypes:Instant\n"+
		"A:SP$ Destroy | ValidTgts$ Permanent | TgtPrompt$ Select target permanent\nOracle:x\n")
	reanimate := card(t, "Name:Fixture Reanimate\nManaCost:2 B\nTypes:Instant\n"+
		"A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouGraveyard | TgtPrompt$ Select target creature card in your graveyard\nOracle:x\n")
	e := handEngine(t, exiler, removal, reanimate)
	bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	return e, bear, state.ObjID(0)
}

func ltbReentryCastAndPass(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	e.askPriority(0)
	castFirst(t, e, "cast")
	return passToTargetAsk(t, e)
}

func ltbReentryPick(t *testing.T, e *Engine, d *decision.Decision, want state.ObjID, what string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == want {
			return o.Index
		}
	}
	t.Fatalf("%s not offered: %+v", what, d.Options)
	return -1
}

func TestLTBTriggerPendingRememberedSurvivesSourceReentry(t *testing.T) {
	e, bear, exiler := ltbReentryEngine(t)

	// ETB: exile the opponent's bear (RememberChanged$ True writes the
	// exiler's persistent Remembered).
	e.G.Players[0].Pool[state.MW] = 1
	e.G.Players[0].Pool[state.MC] = 1
	d := ltbReentryCastAndPass(t, e)
	submitChoices(t, e, ltbReentryPick(t, e, d, bear, "exile target"))
	passUntilStackEmpty(t, e, 8)
	if z := e.G.Obj(bear).Zone; z != state.ZExile {
		t.Fatalf("precondition: bear zone = %v, want exile", z)
	}
	exiler = state.ObjID(0)
	for _, o := range e.G.Zone(state.ZBattlefield, 0) {
		exiler = o
	}
	if got := e.G.Obj(exiler).Remembered; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: exiler Remembered = %+v, want [%d]", got, bear)
	}
	if inc := e.G.Obj(exiler).Incarnation; inc != 1 {
		t.Fatalf("precondition: exiler Incarnation = %d, want 1 (first entry)", inc)
	}

	// Destroy the exiler: its leave trigger queues on the stack.
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MW] = 1
	e.G.Players[0].Pool[state.MC] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Vindicate's target ask, got %+v", d)
	}
	submitChoices(t, e, ltbReentryPick(t, e, d, exiler, "destroy target"))
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision while waiting for the leave trigger to stack")
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("expected priority, got %+v", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
		if e.G.Obj(exiler).Zone == state.ZGraveyard && len(e.G.Stack) > 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the leave trigger never stacked (or already resolved)")
	}
	if z := e.G.Obj(exiler).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: exiler zone = %v, want graveyard", z)
	}
	if got := e.G.Obj(exiler).Remembered; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: the departure keeps the list for pending referents, got %+v", got)
	}
	if inc := e.G.Obj(exiler).Incarnation; inc != 2 {
		t.Fatalf("precondition: exiler Incarnation = %d, want 2 (one entry, one departure)", inc)
	}

	// Re-enter the exiler from the graveyard BEFORE its leave trigger
	// resolves: a CR 400.7 battlefield re-entry, the same direct-event
	// re-entry the Curator regression test drives.
	e.emit(events.Event{Kind: events.MoveZone, Obj: exiler, From: state.ZGraveyard, To: state.ZBattlefield})
	if inc := e.G.Obj(exiler).Incarnation; inc != 3 {
		t.Fatalf("precondition: exiler Incarnation = %d, want 3 after the re-entry", inc)
	}
	if got := e.G.Obj(exiler).Remembered; len(got) != 0 {
		t.Fatalf("the re-entry is a new object: Remembered = %+v, want cleared", got)
	}

	// Now everything resolves. The pending leave trigger (the OLD object's)
	// must still return the bear to the battlefield.
	passUntilStackEmpty(t, e, 12)
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("the exiled bear zone = %v, want battlefield: the pending leave trigger must resolve the old object's remembered set after the source re-entered (CR 400.7)", z)
	}
}
