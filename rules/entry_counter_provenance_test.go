package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestEntryCounterNoticePublishesAdder(t *testing.T) {
	walker := entryCounterWalker(t)
	e, cfg := tokenReplGame(t, 9401, walker)
	id := moveSeededCard(t, e, 0, walker, state.ZHand)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: walker not in hand: %+v", o)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 4 {
		t.Fatalf("precondition: walker entry = %+v, want battlefield with 4 loyalty", o)
	}
	assertEntryCounterLedger(t, e, id, "LOYALTY", 4, 0)
	replayCheck(t, e, cfg)
}

func assertEntryCounterLedger(t *testing.T, e *Engine, id state.ObjID, kind string, amount int32, actor state.PlayerID) {
	t.Helper()
	for _, row := range e.counterAddsThisTurn {
		if row.object.ID == id && row.kind == kind && row.amount == amount && row.actor == actor {
			return
		}
	}
	t.Fatalf("entry %s placement (%d) absent from ledger with seat %d as adder: %+v", kind, amount, actor, e.counterAddsThisTurn)
}

func TestCastEntryCounterProvenanceNotifiesTriggerAndLedger(t *testing.T) {
	ds := tokenReplCorpusCard(t, "Doubling Season")
	walker := entryCounterWalker(t)
	patron := card(t, "Name:Entry Patron\nTypes:Creature\nPT:1/1\n"+
		"T:Mode$ CounterPlayerAddedAll | ValidSource$ Opponent | TriggerZones$ Battlefield | Execute$ Draw\n"+
		"SVar:Draw:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, 9402, []*cards.Card{ds, walker}, []*cards.Card{patron})
	moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	pid := moveSeededCard(t, e, 1, patron, state.ZBattlefield)
	if len(e.G.Obj(pid).Face().Triggers) == 0 {
		t.Fatal("precondition: opponent trigger source has no parsed trigger")
	}
	if o := e.G.Obj(pid); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatal("precondition: patron not on opponent battlefield")
	}
	wid := moveSeededCard(t, e, 0, walker, state.ZHand)
	toMain1(t, e)
	e.priorityRound()
	castSpellOption(t, e, "Entry Walker")
	passPriorityOnce(t, e)
	passPriorityOnce(t, e)
	o := e.G.Obj(wid)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("LOYALTY") != 8 {
		t.Fatalf("precondition: cast walker = %+v, want battlefield with 8 loyalty", o)
	}
	assertEntryCounterLedger(t, e, wid, "LOYALTY", 8, 0)
	if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
		t.Fatalf("opponent CounterPlayerAddedAll trigger did not see doubled entry placement; pending=%d stack=%d", len(e.pendingTriggers), len(e.G.Stack))
	}
	replayCheck(t, e, cfg)
}

func TestEntryBodyCounterFinalEmitPublishesAdder(t *testing.T) {
	creature := entryCounterEtbCreature(t)
	e, cfg := tokenReplGame(t, 9403, creature)
	id := moveSeededCard(t, e, 0, creature, state.ZHand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 2 {
		t.Fatalf("precondition: recipient = %+v, want 2 counters", o)
	}
	assertEntryCounterLedger(t, e, id, "P1P1", 2, 0)
	replayCheck(t, e, cfg)
}

func TestSagaEntryAndTurnLoreCounterUnderDoublingSeason(t *testing.T) {
	ds := tokenReplCorpusCard(t, "Doubling Season")
	saga := card(t, "Name:Triage Saga\nTypes:Enchantment Saga\nK:Chapter:2:DBDraw\nSVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	e, _ := tokenReplGame(t, 9404, ds, saga)
	moveSeededCard(t, e, 0, ds, state.ZBattlefield)
	id := moveSeededCard(t, e, 0, saga, state.ZBattlefield)
	if got := e.G.Obj(id).Counter("LORE"); got != 1 {
		t.Fatalf("Saga enters with %d lore on harness path (not a cast), want baseline 1", got)
	}
	e.advanceSagas(0)
	if got := e.G.Obj(id).Counter("LORE"); got != 2 {
		t.Fatalf("turn-based lore counter under Doubling Season = %d, want 2", got)
	}
}

var _ *cards.Card
