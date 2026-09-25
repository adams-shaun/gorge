package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestChangeZoneAllRememberLKIPreservesPreMoveSnapshot(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	bear := g.Obj(ids["myBear"])
	if bear == nil || bear.Zone != state.ZBattlefield {
		t.Fatalf("precondition: bearer = %#v, want battlefield object", bear)
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: bear.ID, Counter: "P1P1", Amount: 1})
	preMovePower := h.Power(bear.ID)
	postMoveBasePower := int32(bear.Face().Power())
	if preMovePower == postMoveBasePower {
		t.Fatalf("precondition: pre-move power %d must differ from post-move base power %d", preMovePower, postMoveBasePower)
	}
	if bear.Counter("P1P1") != 1 {
		t.Fatalf("precondition: battlefield object's +1/+1 counters = %d, want 1", bear.Counter("P1P1"))
	}

	ability := sa(t, "DB$ ChangeZoneAll | Origin$ Battlefield | Destination$ Graveyard | ChangeType$ Creature | ChangeNum$ 1 | RememberLKI$ True")
	if ability.API != "ChangeZoneAll" {
		t.Fatalf("precondition: API = %q, want ChangeZoneAll", ability.API)
	}
	ctx := &Ctx{Source: ids["myLand"], Controller: 0}
	Resolve(h, ctx, ability)
	if bear.Zone != state.ZGraveyard {
		t.Fatalf("bear zone = %s, want graveyard", bear.Zone)
	}
	if len(ctx.Remembered) != 1 || ctx.Remembered[0].Obj != bear.ID {
		t.Fatalf("Remembered = %v, want moved bearer %d", ctx.Remembered, bear.ID)
	}
	if got := EvalCount(h, ctx, "Remembered$CardPower"); got != preMovePower {
		t.Fatalf("chained Remembered$CardPower = %d, want pre-move power %d", got, preMovePower)
	}
	if got := EvalCount(h, ctx, "Remembered$CardCounters.P1P1"); got != 1 {
		t.Fatalf("chained Remembered$CardCounters.P1P1 = %d, want pre-move counter count 1", got)
	}
}
