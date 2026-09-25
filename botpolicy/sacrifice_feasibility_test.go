package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestSacrificeBotAnswerValidatesAgainstFeasibleOptions(t *testing.T) {
	d := &decision.Decision{Seq: 7, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		ResumeKind: "sacrifice", Options: []decision.Option{{Index: 0, Kind: "sacrifice", Obj: state.ObjID(42)}}}
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("bot sacrifice answer %+v rejected: %v", in, err)
	}
	clamped := Clamp(d, in)
	if err := d.Validate(clamped); err != nil {
		t.Fatalf("clamped bot sacrifice answer %+v rejected: %v", clamped, err)
	}
	if len(in.Choices) != 1 || in.Choices[0] != d.Options[0].Index {
		t.Fatalf("bot choices = %v, want the only feasible sacrifice option", in.Choices)
	}
}
