package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestBlackPantherRememberAmountFeedsGainLife follows Black Panther's
// compiled DBMove -> DBGainLife chain. The move remembers one entry per
// counter, and the chained payoff reads that context through X.
func TestBlackPantherRememberAmountFeedsGainLife(t *testing.T) {
	card, dbMove := corpusSA(t, "Black Panther, Wakandan King", "DBMove")
	_, dbGainLife := corpusSA(t, "Black Panther, Wakandan King", "DBGainLife")
	if dbMove.API != "MoveCounter" || dbGainLife.API != "GainLife" {
		t.Fatalf("compiled SVars = %s -> %s, want MoveCounter -> GainLife", dbMove.API, dbGainLife.API)
	}
	if dbMove.Sub == nil || dbMove.Sub.API != dbGainLife.API {
		t.Fatalf("DBMove sub-ability = %#v, want compiled DBGainLife", dbMove.Sub)
	}

	h := newHost(t, 2)
	pantherID := h.g.AddObject(card, 0).ID
	landID := h.g.AddObject(mkCard(t, "Name:Vibranium Land\nTypes:Land\nOracle:x\n"), 0).ID
	creatureID := h.g.AddObject(mkCard(t, "Name:Target Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0).ID
	for _, id := range []state.ObjID{pantherID, landID, creatureID} {
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	panther := h.g.Obj(pantherID)
	land := h.g.Obj(landID)
	creature := h.g.Obj(creatureID)
	h.Emit(events.Event{Kind: events.CounterChange, Obj: landID, Counter: "P1P1", Amount: 2})
	h.g.Players[0].Life = 20

	if panther.Zone != state.ZBattlefield || land.Zone != state.ZBattlefield || creature.Zone != state.ZBattlefield {
		t.Fatalf("precondition: objects must be on battlefield: panther=%v land=%v creature=%v", panther.Zone, land.Zone, creature.Zone)
	}
	if land.Counter("P1P1") != 2 || creature.Counter("P1P1") != 0 || land.ID == creature.ID {
		t.Fatalf("precondition: land=%d counters, creature=%d counters, ids=%d/%d; want 2/0 and distinct", land.Counter("P1P1"), creature.Counter("P1P1"), land.ID, creature.ID)
	}
	beforeLife := h.g.Players[0].Life
	c := &Ctx{
		Controller:      0,
		Source:          pantherID,
		SVars:           panther.Face().SVars,
		Targets:         []state.Target{{Obj: landID}},
		TargetsPick:     []state.Target{{Obj: creatureID}},
		TargetsPickDone: true,
	}
	// Resolve the two compiled SVars separately so the later compiled
	// DBDraw/DBCleanup tail does not erase Remembered before we inspect it.
	moveOnly := *dbMove
	moveOnly.Sub = nil
	gainOnly := *dbGainLife
	gainOnly.Sub = nil
	Resolve(h, c, &moveOnly)

	if got := land.Counter("P1P1"); got != 0 {
		t.Fatalf("land P1P1 = %d, want 0 after moving both counters", got)
	}
	if got := creature.Counter("P1P1"); got != 2 {
		t.Fatalf("creature P1P1 = %d, want 2 after receiving both counters", got)
	}
	if got := len(c.Remembered); got != 2 {
		t.Fatalf("Remembered length = %d, want 2 moved-counter entries", got)
	}
	Resolve(h, c, &gainOnly)
	if got := h.g.Players[0].Life; got != beforeLife+2 {
		t.Fatalf("life = %d, want %d from compiled DBGainLife's X", got, beforeLife+2)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API MoveCounter") {
			t.Fatalf("compiled MoveCounter did not run: %+v", ev)
		}
	}
}
