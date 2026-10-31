package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedReturnFindsOnlyTheRememberedObject pins CR 400.7 for a delayed
// trigger's remembered object: Nine-Lives Familiar dies with revival
// counters and registers "return it to the battlefield at the beginning of
// the next end step" remembering the card in the graveyard. If another
// effect returns that card (Zombify) and it dies again before the end step,
// the card in the graveyard is a new object and the delayed trigger returns
// nothing. The registration stamps each remembered object's Incarnation
// (state.DelayedTrigger.RememberedStamps) and ChangeZone's Defined$
// DelayTriggerRemembered[LKI] mover skips one that moved since
// (effects.dropDelayedRememberedMoved). dirac K1h pair 4347. The
// control arm has no round trip and must return the Familiar.
func TestDelayedReturnFindsOnlyTheRememberedObject(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		e := handEngine(t, corpusAlternativeCard(t, "Nine-Lives Familiar"))
		id := e.G.Zone(state.ZHand, 0)[0]
		e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 1, 2
		castMode(t, e, id, "")
		finishCast(t, e, id)
		if got := e.G.Obj(id).Counter("REVIVAL"); got != 8 {
			t.Fatalf("precondition: entered with %d revival counters, want 8", got)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
		drainQueuedTriggers(t, e)
		if len(e.G.Delayed) != 1 {
			t.Fatalf("death registered %d delayed triggers, want 1", len(e.G.Delayed))
		}
		if roundTrip {
			// Returned with no revival counters, so its second death
			// registers nothing.
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZBattlefield})
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
			drainQueuedTriggers(t, e)
			if len(e.G.Delayed) != 1 {
				t.Fatalf("roundTrip: %d delayed triggers, want the first death's 1", len(e.G.Delayed))
			}
		}
		passUntil(t, e, func() bool {
			return len(e.G.Delayed) == 0 && len(e.G.Stack) == 0 && e.Pending() != nil
		})
		want := state.ZBattlefield
		if roundTrip {
			want = state.ZGraveyard
		}
		if z := e.G.Obj(id).Zone; z != want {
			t.Fatalf("roundTrip=%v: Familiar zone %v, want %v", roundTrip, z, want)
		}
	}
}
