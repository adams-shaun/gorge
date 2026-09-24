package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func counterAdjPermanent(t *testing.T, h *fakeHost, owner state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, src), owner)
	id := o.ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	if got := h.g.Obj(id); got == nil || got.Zone != state.ZBattlefield {
		t.Fatalf("permanent %d is not on the battlefield", id)
	}
	return id
}

func TestCountDifferentCounterKindsBlitzballStadium(t *testing.T) {
	card, draw := corpusSA(t, "Blitzball Stadium", "TrigDraw")
	if draw.API != "Draw" || draw.Params["NumCards"] != "Count$DifferentCounterKinds_Card.Self" {
		t.Fatalf("Blitzball draw SA = %#v", draw)
	}
	h := newHost(t, 2)
	stadium := h.g.AddObject(card, 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: stadium, From: state.ZLibrary, To: state.ZBattlefield})
	if o := h.g.Obj(stadium); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("Stadium must be on the battlefield")
	}
	// A different permanent's counters must not contribute to Card.Self.
	other := counterAdjPermanent(t, h, 0, "Name:Other\nTypes:Creature\nPT:2/2\nOracle:x\n")
	h.Emit(events.Event{Kind: events.CounterChange, Obj: other, Counter: "AGE", Amount: 1})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: stadium, Counter: "CHARGE", Amount: 3})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: stadium, Counter: "LORE", Amount: 1})
	if h.g.Obj(stadium).Counter("CHARGE") != 3 || h.g.Obj(stadium).Counter("LORE") != 1 || h.g.Obj(other).Counter("AGE") != 1 {
		t.Fatal("counter precondition: two kinds with differing counts on Stadium, third on another permanent")
	}
	// Set up enough library cards to distinguish kinds (2) from total counters (4).
	for i := 0; i < 5; i++ {
		id := h.g.AddObject(mkCard(t, "Name:Drawn\nTypes:Instant\nOracle:x\n"), 0).ID
		h.Emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZLibrary})
	}
	if got := len(h.g.Zone(state.ZLibrary, 0)); got != 5 {
		t.Fatalf("library setup = %d", got)
	}
	Resolve(h, &Ctx{Source: stadium, Controller: 0}, draw)
	if got := len(h.g.Zone(state.ZHand, 0)); got != 2 {
		t.Fatalf("Blitzball drew %d, want two kinds", got)
	}
	for _, expr := range []string{"Count$DifferentCounterKinds_Card.SelfExtra", "Count$DifferentCounterKinds_Unknown", "Count$DifferentCounterKinds_"} {
		if n, ok := EvalCountOK(h, &Ctx{Source: stadium, Controller: 0}, expr); ok {
			t.Errorf("unsupported %s resolved to %d", expr, n)
		}
	}
}

func TestPlayerCountOpponentsControlsCreaturePower(t *testing.T) {
	card, token := corpusSA(t, "Summon: Yojimbo", "DBToken")
	const expr = "PlayerCountOpponents$HasPropertycontrolsCreature.powerGE4"
	if token.API != "Token" || token.Params["TokenAmount"] != expr {
		t.Fatalf("Yojimbo token SA = %#v", token)
	}
	h := newHost(t, 3)
	h.g.Tokens = testutil.CorpusRegistry(t).Tokens
	source := h.g.AddObject(card, 0).ID
	h.Emit(events.Event{Kind: events.MoveZone, Obj: source, From: state.ZLibrary, To: state.ZBattlefield})
	if h.g.Obj(source).Zone != state.ZBattlefield {
		t.Fatal("Yojimbo not on battlefield")
	}
	low := counterAdjPermanent(t, h, 1, "Name:Small\nTypes:Creature\nPT:3/3\nOracle:x\n")
	high := counterAdjPermanent(t, h, 0, "Name:Mine\nTypes:Creature\nPT:5/5\nOracle:x\n")
	if h.Power(low) != 3 || h.Power(high) != 5 || h.Power(low) == h.Power(high) || h.g.Obj(low).Zone != state.ZBattlefield || h.g.Obj(high).Zone != state.ZBattlefield {
		t.Fatal("power-boundary precondition: 3 and 5 power creatures on battlefield")
	}
	c := &Ctx{Source: source, Controller: 0}
	check := func(want int32) {
		t.Helper()
		if n, ok := EvalCountOK(h, c, expr); !ok || n != want {
			t.Fatalf("Yojimbo opponents = (%d,%v), want %d", n, ok, want)
		}
		before := 0
		for _, ev := range h.log {
			if ev.Kind == events.TokenCreate && ev.Text == "c_a_treasure_sac" {
				before++
			}
		}
		Resolve(h, c, token)
		after := 0
		for _, ev := range h.log {
			if ev.Kind == events.TokenCreate && ev.Text == "c_a_treasure_sac" {
				after++
			}
		}
		if int32(after-before) != want {
			t.Fatalf("Yojimbo made %d Treasures, want %d", after-before, want)
		}
	}
	check(0) // Our own 5/5 never counts; the opponent has only a 3/3.
	h.Emit(events.Event{Kind: events.CounterChange, Obj: low, Counter: "P1P1", Amount: 1})
	if h.Power(low) != 4 {
		t.Fatalf("boundary precondition: boosted power = %d, want 4", h.Power(low))
	}
	check(1)
	another := counterAdjPermanent(t, h, 2, "Name:Large\nTypes:Creature\nPT:5/5\nOracle:x\n")
	if h.g.Obj(another).Zone != state.ZBattlefield || h.Power(another) != 5 {
		t.Fatal("second opponent's qualifying creature not on battlefield")
	}
	check(2)
	for _, bad := range []string{"PlayerCountOpponents$HasPropertycontrolsCreature.powerGE", "PlayerCountOpponents$HasPropertycontrolsCreature.powerGE5", "PlayerCountOpponents$HasPropertycontrolsCreature.powerGE4 extra"} {
		if n, ok := EvalCountOK(h, c, bad); ok {
			t.Errorf("unsupported %s resolved to %d", bad, n)
		}
	}
}
