package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 603.3a: the source's controller controls a triggered ability, whatever the
// script's TriggerController$ names, so a "you" in its effect reads that player.
func TestTriggerControllerParamLeavesTheSourcesController(t *testing.T) {
	t.Parallel()
	const watcher = "Name:Controller Witness\nTypes:Enchantment\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | TriggerZones$ Battlefield | TriggerController$ TriggeredCardController | Execute$ TrigLife | TriggerDescription$ x\n" +
		"SVar:TrigLife:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e := stealEngine(t, 913)
	witness := onBoard(t, e, 1, watcher)
	bear := onBoardReady(t, e, 1, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if e.G.Obj(bear).Zone != state.ZBattlefield || e.G.Obj(bear).Controller != 1 {
		t.Fatalf("precondition: bear = %+v, want battlefield under seat 1", e.G.Obj(bear))
	}
	e.emit(events.Event{Kind: events.ControlChange, Obj: bear, Player: 0})
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("precondition: control change left bear controlled by %d, want seat 0", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(bear).Zone != state.ZGraveyard || e.G.Obj(bear).Controller != 1 {
		t.Fatalf("precondition: departed bear = %+v, want owner's graveyard after a seat-0-controlled departure", e.G.Obj(bear))
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != witness {
		t.Fatalf("pending triggers = %+v, want the witness's one trigger", e.pendingTriggers)
	}
	if got := e.pendingTriggers[0].Controller; got != 1 {
		t.Fatalf("trigger controller = %d, want the witness's controller seat 1 (the departing card's was seat 0)", got)
	}
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Controller != 1 {
		t.Fatalf("trigger stack = %v, want one stack object controlled by seat 1", e.G.Stack)
	}
	life := e.G.Players[1].Life
	e.resolveTop()
	if e.G.Players[1].Life != life+1 || e.G.Players[0].Life != 20 {
		t.Fatalf("resolution life = [%d %d], want only seat 1 to gain 1", e.G.Players[0].Life, e.G.Players[1].Life)
	}
}

// An Aura's "when enchanted creature dies" ability is the printed carrier.
func TestAuraDiesTriggerBelongsToTheAurasController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Grizzly Bears")}, []*cards.Card{lookup(t, reg, "Fungal Fortitude")})
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	aura := moveByName(t, e, 1, "Fungal Fortitude", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bears}})

	e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZBattlefield, To: state.ZGraveyard})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("%d pending triggers, want the Aura's", len(e.pendingTriggers))
	}
	if pt := e.pendingTriggers[0]; pt.Source != aura || pt.Controller != 1 {
		t.Fatalf("trigger source %d controller %d, want the Aura %d controlled by seat 1", pt.Source, pt.Controller, aura)
	}
}
