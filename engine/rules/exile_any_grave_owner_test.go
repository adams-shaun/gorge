package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature that dies under another player's control goes to its owner's
// graveyard (CR 404.1, 700.4). Its dies trigger's ExileAnyGrave cost may exile
// the card from any graveyard, so the controller is still offered to pay it.
func TestExileAnyGraveCostReachesTheOwnersGraveyard(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9733, ppFiller)
	worm := onBoard(t, e, 0, "Name:Grave Worm\nManaCost:1 B\nTypes:Creature Worm\nPT:2/2\n"+
		"T:Mode$ ChangesZone | ValidCard$ Card.Self | Origin$ Battlefield | Destination$ Graveyard | Execute$ TrigAgain | TriggerDescription$ When this dies, you may exile it. When you do, draw a card.\n"+
		"SVar:TrigAgain:AB$ ImmediateTrigger | Cost$ ExileAnyGrave<1/Card.TriggeredNewCard> | Execute$ TrigDrawIt | TriggerDescription$ When you do, draw a card.\n"+
		"SVar:TrigDrawIt:DB$ Draw | NumCards$ 1\nOracle:x\n")
	e.emit(events.Event{Kind: events.ControlChange, Obj: worm, Player: 1})
	e.emit(events.Event{Kind: events.MoveZone, Obj: worm, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	if o := e.G.Obj(worm); o.Zone != state.ZGraveyard || o.Owner != 0 {
		t.Fatalf("precondition: worm in zone %v owner %d, want its owner's graveyard", o.Zone, o.Owner)
	}
	e.putTriggersOnStack()
	e.pending = nil
	e.resolveTop()
	d := waitForCostWindow(t, e)
	if d.Player != 1 || d.Options[0].Kind != "trigger_cost_pay" {
		t.Fatalf("the controller was not offered to pay the exile: player %d options %+v", d.Player, d.Options)
	}
	submitChoices(t, e, d.Options[0].Index)
	if z := e.G.Obj(worm).Zone; z != state.ZExile {
		t.Fatalf("paying left the worm in %v", z)
	}
}
