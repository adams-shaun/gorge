package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Damage dealt simultaneously is one time the recipient is dealt damage (CR
// 510.2): FirstTime$ on DamageDoneOnce reads the whole batch, so two blockers
// hitting a Hercules-shaped creature at once give it counters for both hits,
// not only for the first Damage event of the batch.
func TestDamageDoneOnceFirstTimeSumsTheSimultaneousBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	src := "Name:First-Hit Counter\nManaCost:1 G\nTypes:Creature\nPT:2/9\n" +
		"T:Mode$ DamageDoneOnce | ValidTarget$ Card.Self | FirstTime$ True | TriggerZones$ Battlefield | Execute$ GrowByHits\n" +
		"SVar:GrowByHits:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ HitTotal\n" +
		"SVar:HitTotal:TriggerCount$DamageAmount\nOracle:x\n"
	e, cfg := combatTriggerBoard(t, reg, nil, []string{src}, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	carrier := findBattlefield(t, e, 0, "First-Hit Counter", 0)
	if o := e.G.Obj(carrier); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("trigger carrier = %+v, want battlefield object", o)
	}

	e.BeginDamageBatch()
	e.emit(events.Event{Kind: events.Damage, Obj: carrier, Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Obj: carrier, Amount: 3})
	e.EndDamageBatch()
	if got := e.G.Obj(carrier).Damage; got != 5 {
		t.Fatalf("damage after the batch = %d, want 5 (both events must apply)", got)
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending triggers after the batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 5 {
		t.Fatalf("counters after a two-hit first batch = %d, want 5", got)
	}

	// A later batch the same turn is no longer the first time.
	e.BeginDamageBatch()
	e.emit(events.Event{Kind: events.Damage, Obj: carrier, Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: carrier, Amount: 1})
	e.EndDamageBatch()
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pending triggers after the later batch = %d, want 0", len(e.pendingTriggers))
	}
	replayCheck(t, e, cfg)
}
