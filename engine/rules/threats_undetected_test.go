package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestThreatsUndetectedDifferentPowers drives the corpus Threats Undetected
// (ChangeZone DifferentPower$ True): the offered search refuses a pair of
// same-power creatures and accepts creatures of pairwise different powers.
func TestThreatsUndetectedDifferentPowers(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Threats Undetected", "Hill Giant", "Serra Angel")
	// Force the 3/3 and the 4/4 into the library beside the library's 2/2
	// Grizzly Bears, wherever the opening draw left them.
	hill := searchMoveByName(t, e, "Hill Giant", state.ZLibrary)
	angel := searchMoveByName(t, e, "Serra Angel", state.ZLibrary)
	_, d := castSearchSpell(t, e, "Threats Undetected")
	if d == nil || d.Kind != decision.KChoose || d.Player != 0 {
		t.Fatalf("cast pending = %+v, want the caster's search ask", d)
	}
	optFor := func(id state.ObjID) *decision.Option {
		for i := range d.Options {
			if d.Options[i].Obj == id {
				return &d.Options[i]
			}
		}
		return nil
	}
	hillOpt, angelOpt := optFor(hill), optFor(angel)
	if hillOpt == nil || angelOpt == nil {
		t.Fatalf("the 3/3 and 4/4 are not offered: %+v", d.Options)
	}
	// Precondition: the other creature options are 2-power bears sharing one
	// group, distinct from the 3/3's and the 4/4's.
	var bears []decision.Option
	for _, o := range d.Options {
		if o.Obj != hill && o.Obj != angel {
			bears = append(bears, o)
		}
	}
	if len(bears) < 2 {
		t.Fatalf("fixture offers %d bears, need 2", len(bears))
	}
	for _, b := range bears {
		if b.Group == "" || b.Group != bears[0].Group {
			t.Fatalf("bear option group %q, want the shared power group %q", b.Group, bears[0].Group)
		}
	}
	if hillOpt.Group == "" || hillOpt.Group == angelOpt.Group || hillOpt.Group == bears[0].Group || angelOpt.Group == bears[0].Group {
		t.Fatalf("power groups not distinct: bear %q hill %q angel %q", bears[0].Group, hillOpt.Group, angelOpt.Group)
	}
	if d.Max != 3 {
		t.Fatalf("Max = %d, want 3 distinct powers", d.Max)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{bears[0].Index, bears[1].Index}}); err == nil {
		t.Fatal("Validate accepted two same-power bears")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{bears[0].Index, hillOpt.Index, angelOpt.Index}}); err != nil {
		t.Fatalf("distinct-power answer refused: %v", err)
	}
	replayCheck(t, e, cfg)
}
