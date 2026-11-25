package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A commander parked (CR 903.9b bounce) inside a batch window and then declined replays its move
// under the parked window's snapshot, which records the departure in the
// window's list. The decline's restore must retire that list like every other
// window end, or the commander -- reanimated under the same id -- stays hidden
// from a later death's "another creature" intervening-if.
func TestLifelineCmdZoneDeclineDoesNotLeaveAStaleDeparture(t *testing.T) {
	t.Parallel()
	e, _ := cmdZoneGame(t, [][]string{{tinyCmdSrc}, {}})
	cmd := fieldCommander(t, e, 0, 0)
	lifeline := onBoard(t, e, 0, lbLifeline)
	bear := onBoard(t, e, 0, lbBear)

	e.BatchDepartures([]state.ObjID{cmd})
	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZBattlefield, To: state.ZHand})
	e.EndBatchDepartures()
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone {
		t.Fatalf("precondition: pending = %+v, want the commander_zone decision", d)
	}
	submit(t, e, 1) // decline: the commander is bounced to hand
	if o := e.G.Obj(cmd); o.Zone != state.ZHand {
		t.Fatalf("precondition: declined commander in %v, want hand", o.Zone)
	}
	if e.triggerBefore != nil {
		t.Fatalf("precondition: the decline left a window open")
	}
	if len(e.windowDepartures) != 0 {
		t.Fatalf("the declined move left %v in the retired window's departure list", e.windowDepartures)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: cmd, From: state.ZHand, To: state.ZBattlefield})
	if o := e.G.Obj(cmd); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: returned commander in %v, want battlefield", o.Zone)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(bear); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear in %v, want graveyard", o.Zone)
	}
	if n := lbQueuedFor(e, lifeline); n != 1 {
		t.Fatalf("Lifeline queued %d triggers with the commander on the battlefield, want 1", n)
	}
}
