package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// auntMayText is Aunt May's trigger shape (inline; the corpus script is not
// committed): "Whenever another creature you control enters, you gain 1
// life. If it's a Spider, put a +1/+1 counter on it." The counter rides the
// dotted `Defined$ TriggeredCardLKICopy.Spider` selector.
const auntMayText = `Name:Aunt May
ManaCost:W
Types:Legendary Creature Human Citizen
PT:0/2
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other+YouCtrl | TriggerZones$ Battlefield | Execute$ OnGainLife | TriggerDescription$ x
SVar:OnGainLife:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SubAbility$ PutCounter
SVar:PutCounter:DB$ PutCounter | Defined$ TriggeredCardLKICopy.Spider | CounterType$ P1P1 | CounterNum$ 1
Oracle:x
`

// enterUnderAuntMay moves a creature from hand onto the battlefield under
// Aunt May's controller and resolves her trigger, returning May, the entrant
// and the life gained.
func enterUnderAuntMay(t *testing.T, entrant string) (*Engine, state.ObjID, state.ObjID, int32) {
	t.Helper()
	e := combatEngine(t)
	may := onBoardCard(t, e, 0, card(t, auntMayText))
	o := e.G.AddObject(card(t, entrant), 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	life := e.G.Players[0].Life
	before := len(e.pendingTriggers)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield, Player: 0})
	if got := e.G.Obj(o.ID); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("precondition: entrant not on the battlefield: %+v", got)
	}
	if len(e.pendingTriggers)-before != 1 {
		t.Fatalf("precondition: queued triggers = %d, want Aunt May's one", len(e.pendingTriggers)-before)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want Aunt May's trigger", e.G.Stack)
	}
	resolveAll(t, e)
	return e, may, o.ID, e.G.Players[0].Life - life
}

// TestAuntMaySpiderEntrantGetsTheCounter: a Spider entering gets the
// +1/+1 counter and Aunt May does not.
func TestAuntMaySpiderEntrantGetsTheCounter(t *testing.T) {
	t.Parallel()
	e, may, spider, gained := enterUnderAuntMay(t, "Name:Test Spider\nTypes:Creature Spider\nPT:1/1\nOracle:x\n")
	if gained != 1 {
		t.Fatalf("life gained = %d, want 1 (the trigger must have resolved)", gained)
	}
	if got := e.G.Obj(spider).Counter("P1P1"); got != 1 {
		t.Fatalf("Spider P1P1 = %d, want 1", got)
	}
	if got := e.G.Obj(may).Counter("P1P1"); got != 0 {
		t.Fatalf("Aunt May P1P1 = %d, want 0", got)
	}
}

// TestAuntMayNonSpiderEntrantGetsNoCounter: a non-Spider entering gains the
// life but puts NO counter anywhere.
func TestAuntMayNonSpiderEntrantGetsNoCounter(t *testing.T) {
	t.Parallel()
	e, may, bear, gained := enterUnderAuntMay(t, "Name:Test Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if gained != 1 {
		t.Fatalf("life gained = %d, want 1 (the trigger must have resolved)", gained)
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("Bear P1P1 = %d, want 0", got)
	}
	if got := e.G.Obj(may).Counter("P1P1"); got != 0 {
		t.Fatalf("Aunt May P1P1 = %d, want 0 (counter fell back to Self)", got)
	}
}
