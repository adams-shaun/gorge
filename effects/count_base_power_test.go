package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestEvalCountCardBasePowerAndDifferenceOperand(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	source := ids["myBear"]
	events.Apply(g, events.Event{Kind: events.CounterChange, Obj: source, Counter: "P1P1", Amount: 1})
	ctx := &Ctx{Source: source, Controller: 0, SVars: map[string]string{
		"X": "Count$CardPower/Minus.Count$CardBasePower",
	}}
	if o := g.Obj(source); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source is not on battlefield: %+v", o)
	}
	base, ok := EvalCountOK(h, ctx, "Count$CardBasePower")
	if !ok || base != 2 {
		t.Fatalf("CardBasePower = %d, resolved=%v; want printed face 2", base, ok)
	}
	power, ok := EvalCountOK(h, ctx, "Count$CardPower")
	if !ok || power != 3 {
		t.Fatalf("CardPower = %d, resolved=%v; want 3", power, ok)
	}
	difference, ok := EvalCountOK(h, ctx, "SVar$X")
	if !ok || difference != 1 {
		t.Fatalf("CardPower minus CardBasePower = %d, resolved=%v; want 1", difference, ok)
	}
}
