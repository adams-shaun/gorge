package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// quilledGreatwurmGraveyardEngine puts the real corpus Quilled Greatwurm in
// seat 0's graveyard and two Grizzly Bears on the battlefield carrying
// countsA and countsB +1/+1 counters, with {C}{C}{C}{C}{G}{G} floating. It returns
// the engine, the Greatwurm and the two Bears.
func quilledGreatwurmGraveyardEngine(t *testing.T, countsA, countsB int32) (*Engine, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t,
		corpusAlternativeCard(t, "Quilled Greatwurm"),
		corpusAlternativeCard(t, "Grizzly Bears"),
		corpusAlternativeCard(t, "Grizzly Bears"),
	)
	hand := e.G.Zone(state.ZHand, 0)
	wurm, bearA, bearB := hand[0], hand[1], hand[2]
	placeOnBattlefield(t, e, bearA)
	placeOnBattlefield(t, e, bearB)
	e.emit(events.Event{Kind: events.MoveZone, Obj: wurm, From: state.ZHand, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bearA, Counter: "P1P1", Amount: countsA})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bearB, Counter: "P1P1", Amount: countsB})
	e.pending = nil
	addMana(t, e, 0, "CCCCGG")
	return e, wurm, bearA, bearB
}

func greatwurmGraveyardCast(e *Engine, wurm state.ObjID) int {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return -1
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == wurm {
			return o.Index
		}
	}
	return -1
}

// TestQuilledGreatwurmCountersFromAmongCreatures pins Quilled Greatwurm's
// graveyard permission ("by removing six counters from among creatures you
// control", RaiseCost$ RemoveAnyCounter<6/Any/Creature>): the six counters
// are a TOTAL across the payer's creatures (CR 118.3, 601.2h), so two Bears
// carrying three counters each make the cast payable and the payment takes
// counters from both.
func TestQuilledGreatwurmCountersFromAmongCreatures(t *testing.T) {
	t.Parallel()
	e, wurm, bearA, bearB := quilledGreatwurmGraveyardEngine(t, 3, 3)
	idx := greatwurmGraveyardCast(e, wurm)
	if idx < 0 {
		t.Fatalf("graveyard cast not offered with 3+3 counters among two creatures: pending=%+v", e.Pending())
	}
	submitChoices(t, e, idx)
	// Six one-unit answers; prefer Bear A until it is empty, then Bear B.
	for i := 0; i < 12; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			break
		}
		pick := -1
		for _, o := range d.Options {
			if o.Kind != "subcounter" {
				continue
			}
			if o.Obj == bearA || pick < 0 {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("counter-removal ask without a subcounter option: %+v", d)
		}
		submitChoices(t, e, pick)
	}
	if o := e.G.Obj(wurm); o == nil || o.Zone != state.ZStack {
		t.Fatalf("Greatwurm zone after payment = %v, want stack; pending=%+v", o.Zone, e.Pending())
	}
	if a, b := e.G.Obj(bearA).Counter("P1P1"), e.G.Obj(bearB).Counter("P1P1"); a != 0 || b != 0 {
		t.Fatalf("counters left after payment A/B = %d/%d, want 0/0", a, b)
	}
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(wurm); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Greatwurm zone after resolution = %v, want battlefield", o.Zone)
	}
}

// TestQuilledGreatwurmCountersAmongShortfallNotOffered is the boundary: five
// counters among the payer's creatures cannot pay six, so the cast stays
// unoffered.
func TestQuilledGreatwurmCountersAmongShortfallNotOffered(t *testing.T) {
	t.Parallel()
	e, wurm, _, _ := quilledGreatwurmGraveyardEngine(t, 3, 2)
	if idx := greatwurmGraveyardCast(e, wurm); idx >= 0 {
		t.Fatalf("graveyard cast offered with only 5 counters among creatures")
	}
}
