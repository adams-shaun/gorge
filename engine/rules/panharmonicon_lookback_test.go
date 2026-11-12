package rules

// CR 603.10a meets CR 603.2d: a leaves-the-battlefield / sacrifice walk looks
// back at the game immediately before the event, so the Panharmonicon statics
// that decide how many times such a trigger fires are read off that pre-event
// board. The tests pin both halves of the defect: a host that dies earlier in
// the same batch (or is the dying permanent itself) still doubles, and the
// entering shape (StPanhETB) does not double a graveyard-leave trigger caused
// by the same graveyard -> battlefield move.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// panhSelfEcho is a colorless creature whose only trigger is its own death
// and whose Panharmonicon static matches that trigger's source. The dying
// permanent itself must double its own leaves-the-battlefield trigger.
const panhSelfEcho = "Name:Self-echo host\nTypes:Artifact Creature Golem\nPT:1/1\n" +
	"S:Mode$ Panharmonicon | ValidCard$ Card.Self | Description$ If a permanent entering the battlefield causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ Gain | TriggerDescription$ When this dies, you gain 1 life.\n" +
	"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// panhBatchHost is the host side of the batch case: it doubles every colorless
// permanent's triggers but carries no trigger of its own.
const panhBatchHost = "Name:Panh batch host\nTypes:Artifact Creature Golem\nPT:1/1\n" +
	"S:Mode$ Panharmonicon | ValidCard$ Card.YouCtrl+Colorless | Description$ x\nOracle:x\n"

// panhDeathWatcher is the trigger side of the batch case: a colorless death
// trigger the host doubles.
const panhDeathWatcher = "Name:Death watcher\nTypes:Artifact Creature Golem\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ Gain | TriggerDescription$ When this dies, you gain 1 life.\n" +
	"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// panhHasStatic reports whether id carries a live Mode$ Panharmonicon static,
// the precondition every assertion in this file depends on.
func panhHasStatic(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: host %d is not on the battlefield (obj %+v)", id, o)
	}
	for _, st := range o.Face().Statics {
		if st.Mode == "Panharmonicon" {
			return
		}
	}
	t.Fatalf("precondition: host %d carries no Mode$ Panharmonicon static: %+v", id, o.Face().Statics)
}

// TestPanharmoniconHostDyingInBatchStillDoubles pins CR 603.10a against the
// live-board read: the doubling statics come from the look-back board, so a
// host that is itself dying, or that already died earlier in the same
// battlefield-order batch, still doubles the leaves-the-battlefield trigger.
// The watcher-first control proves battlefield order no longer decides.
func TestPanharmoniconHostDyingInBatchStillDoubles(t *testing.T) {
	t.Parallel()

	t.Run("host is the dying permanent itself", func(t *testing.T) {
		t.Parallel()
		e := layerEngine(t)
		id := onBoard(t, e, 0, panhSelfEcho)
		panhHasStatic(t, e, id)
		if got := e.G.Players[0].Life; got != 20 {
			t.Fatalf("precondition: seat 0 life = %d, want 20", got)
		}
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
		e.checkStateBased()
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("precondition: the host is in %s, want the graveyard", z)
		}
		e.Advance()
		drainTriggers(t, e)
		if got := e.G.Players[0].Life; got != 22 {
			t.Fatalf("the dying host's own death trigger gained %d life, want 2 (the look-back board still has the host)", got-20)
		}
	})

	t.Run("host first or last in the batch", func(t *testing.T) {
		t.Parallel()
		for _, hostFirst := range []bool{true, false} {
			e := layerEngine(t)
			var host, watcher state.ObjID
			if hostFirst {
				host = onBoard(t, e, 0, panhBatchHost)
				watcher = onBoard(t, e, 0, panhDeathWatcher)
			} else {
				watcher = onBoard(t, e, 0, panhDeathWatcher)
				host = onBoard(t, e, 0, panhBatchHost)
			}
			panhHasStatic(t, e, host)
			if got := e.G.Players[0].Life; got != 20 {
				t.Fatalf("precondition: seat 0 life = %d, want 20", got)
			}
			// ONE DestroyAll -> one simultaneous departure batch, whose members
			// move in battlefield order. The watcher's death trigger looks back
			// at the pre-batch board, so the host is a live static either way.
			effects.Resolve(e, &effects.Ctx{Source: host, Controller: 0},
				&cards.SA{Kind: "SP", API: "DestroyAll", Params: map[string]string{"ValidCards": "Creature"}})
			for _, id := range []state.ObjID{host, watcher} {
				if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
					t.Fatalf("precondition: %d is in %s after DestroyAll, want the graveyard", id, z)
				}
			}
			e.Advance()
			drainTriggers(t, e)
			if got := e.G.Players[0].Life; got != 22 {
				t.Fatalf("hostFirst=%v: the watcher's death trigger gained %d life, want 2 (host battlefield order must not matter)", hostFirst, got-20)
			}
		}
	})
}

// TestPanharmoniconEnteringShapeDoesNotDoubleGraveyardLeave pins the decision
// for the same-move graveyard-leave echo: Starfield Vocalist's entering shape
// ("if a permanent entering the battlefield causes ...") doubles the entering
// permanent's ETB trigger, but not Willow Geist's "whenever one or more cards
// leave your graveyard" trigger -- that ability is caused by the departure
// from the graveyard, not by the entry (CR 603.2d/603.10a). dirac queues the
// leave at detach, before the destination is known, and stays single.
func TestPanharmoniconEnteringShapeDoesNotDoubleGraveyardLeave(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	starfield := onBoardCard(t, e, 0, corpusCard(t, "Starfield Vocalist"))
	panhHasStatic(t, e, starfield)
	geist := onBoardCard(t, e, 0, corpusCard(t, "Willow Geist"))
	w := e.G.Obj(geist)
	if w == nil || w.Zone != state.ZBattlefield || len(w.Face().Triggers) == 0 ||
		w.Face().Triggers[0].ParamStr(cards.PKOrigin) != "Graveyard" {
		t.Fatalf("precondition: Willow Geist is not a graveyard-leave watcher on the battlefield: %+v", w)
	}
	if w.Counter("P1P1") != 0 {
		t.Fatalf("precondition: Willow Geist starts with %d +1/+1 counters, want 0", w.Counter("P1P1"))
	}
	// The card that moves is the entering permanent (its own ETB gains 4 life);
	// the graveyard-leave trigger is Willow Geist's, on the battlefield.
	missionary := e.G.AddObject(corpusCard(t, "Lone Missionary"), 0)
	missionary.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), missionary.ID))
	if o := e.G.Obj(missionary.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Lone Missionary is not in the graveyard: %+v", o)
	}
	life := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.MoveZone, Obj: missionary.ID, From: state.ZGraveyard, To: state.ZBattlefield})
	if o := e.G.Obj(missionary.ID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Lone Missionary did not enter the battlefield: %+v", o)
	}
	e.Advance()
	drainTriggers(t, e)
	if got := e.G.Players[0].Life - life; got != 8 {
		t.Fatalf("the entering shape gained %d life, want 8 (the ETB trigger is doubled: 2 x 4)", got)
	}
	if got := e.G.Obj(geist).Counter("P1P1"); got != 1 {
		t.Fatalf("the graveyard-leave trigger put %d +1/+1 counters on Willow Geist, want 1 (the entering shape does not double it)", got)
	}
}
