package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mesmeric Fiend's shape (CR 400.7): the leave trigger's body returns the
// remembered card and its SubAbility cleanup clears the source's list. When the
// source re-entered while the trigger waited, the trigger belongs to the OLD
// object: it returns the old object's remembered card from its snapshot, and
// its cleanup must not forget what the NEW object remembered meanwhile (the
// Fiend's second ETB exile), or the new object's own leave trigger returns
// nothing. Fixtures are inline.
func TestLTBCleanupKeepsReenteredSourceRemembered(t *testing.T) {
	exiler := card(t, "Name:Fixture Fiend\nManaCost:1 B\nTypes:Creature Horror\nPT:1/1\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile\n"+
		"SVar:TrigExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Creature.OppCtrl | TgtPrompt$ Select target creature an opponent controls | RememberChanged$ True\n"+
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Card.Self | Execute$ OnLeave | TriggerDescription$ When CARDNAME leaves the battlefield, return the exiled card.\n"+
		"SVar:OnLeave:DB$ ChangeZone | Origin$ Exile | Destination$ Hand | Defined$ Remembered | SubAbility$ DBCleanup\n"+
		"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n"+
		"Oracle:x\n")
	removal := card(t, "Name:Vindicate\nManaCost:1 B W\nTypes:Instant\n"+
		"A:SP$ Destroy | ValidTgts$ Permanent | TgtPrompt$ Select target permanent\nOracle:x\n")
	e := handEngine(t, exiler, removal)
	bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	second := onBoard(t, e, 0, "Name:Second Bear\nManaCost:1 G U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MC] = 1
	d := ltbReentryCastAndPass(t, e)
	submitChoices(t, e, ltbReentryPick(t, e, d, bear, "exile target"))
	passUntilStackEmpty(t, e, 8)
	var fiend state.ObjID
	for _, o := range e.G.Zone(state.ZBattlefield, 0) {
		fiend = o
	}
	if got := e.G.Obj(fiend).Remembered; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: Remembered = %+v, want [%d]", got, bear)
	}

	// Destroy the Fiend; its leave trigger waits on the stack.
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MW] = 1
	e.G.Players[0].Pool[state.MC] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Vindicate's target ask, got %+v", d)
	}
	submitChoices(t, e, ltbReentryPick(t, e, d, fiend, "destroy target"))
	for i := 0; i < 8 && !(e.G.Obj(fiend).Zone == state.ZGraveyard && len(e.G.Stack) > 0); i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("expected priority, got %+v", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if len(e.G.Stack) == 0 || e.G.Obj(fiend).Zone != state.ZGraveyard {
		t.Fatal("precondition: the leave trigger must be waiting with the Fiend in the graveyard")
	}

	// Re-enter (a new object) and let it remember a second card, as its own
	// ETB resolving ahead of the old leave trigger would.
	e.emit(events.Event{Kind: events.MoveZone, Obj: fiend, From: state.ZGraveyard, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Choose, Obj: fiend, Counter: "remembered", IDs: []state.ObjID{second}})
	if got := e.G.Obj(fiend).Remembered; len(got) != 1 || got[0].Obj != second {
		t.Fatalf("precondition: re-entered Remembered = %+v, want [%d]", got, second)
	}

	passUntilStackEmpty(t, e, 12)
	if z := e.G.Obj(bear).Zone; z != state.ZHand {
		t.Fatalf("old exiled card zone = %v, want hand (the old leave trigger's snapshot)", z)
	}
	if got := e.G.Obj(fiend).Remembered; len(got) != 1 || got[0].Obj != second {
		t.Fatalf("the old trigger's cleanup cleared the re-entered object's Remembered: %+v, want [%d]", got, second)
	}
}
