package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Ring-bearer pick (CR 701.54a; effRingTemptsYou's "ring_bearer"
// KChoose) is an ordinary object pick, the kernel's ChGenRingBearer lowering:
// choose:other over the chosen creature, and never an unknown kind.
func TestRingBearerChooseIsKnown(t *testing.T) {
	g := &GorgeSide{E: &rules.Engine{G: &state.Game{}}}
	g.id.inst = make([]Ref, 8)
	g.id.inst[7] = Ref(100)
	d := &decision.Decision{Kind: decision.KChoose}
	o := &decision.Option{Kind: "ring_bearer", Obj: 7}

	if len(g.Stats.Unknown) != 0 {
		t.Fatalf("precondition: Unknown already holds %v", g.Stats.Unknown)
	}
	got, ok := g.optionAction(d, o)
	if !ok {
		t.Fatalf("ring_bearer option has no canonical form")
	}
	want := Pack(ActChoose+chooseOther, Ref(100), RefNone, 0)
	if got != want {
		t.Errorf("ring_bearer canon = %#v, want %#v", got, want)
	}
	if len(g.Stats.Unknown) != 0 {
		t.Errorf("ring_bearer counted as unknown: %v", g.Stats.Unknown)
	}
}
