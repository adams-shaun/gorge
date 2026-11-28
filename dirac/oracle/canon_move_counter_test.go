package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MoveCounter amount pick (engine/effects/counters.go's "move_counter"
// KChoose, CR 122.5; Forgotten Ancient) is the kernel's ChMoveCounterAny
// lowering: choose:other with the option index as the mode, as
// dirac/canon/translate.go packs it, and never an unknown kind.
func TestMoveCounterChooseIsKnown(t *testing.T) {
	g := &GorgeSide{E: &rules.Engine{G: &state.Game{}}}
	d := &decision.Decision{Kind: decision.KChoose, ResumeKind: "move_counter"}
	for i := 0; i <= 3; i++ {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "move_counter", Label: string(rune('0' + i)), Amount: i})
	}

	if len(g.Stats.Unknown) != 0 {
		t.Fatalf("precondition: Unknown already holds %v", g.Stats.Unknown)
	}
	for i := range d.Options {
		o := &d.Options[i]
		got, ok := g.optionAction(d, o)
		if !ok {
			t.Fatalf("move_counter option %d has no canonical form", i)
		}
		want := Pack(ActChoose+chooseOther, RefNone, RefNone, uint8(i))
		if got != want {
			t.Errorf("move_counter option %d canon = %#v, want %#v", i, got, want)
		}
	}
	if len(g.Stats.Unknown) != 0 {
		t.Errorf("move_counter counted as unknown: %v", g.Stats.Unknown)
	}
}
