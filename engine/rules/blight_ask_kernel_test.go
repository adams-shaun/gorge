package rules

// The blight pick, answered through the resolution kernel on a real engine.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBlightTwoCreaturesAsksAndAppliesTheAnswerKernel: with two eligible
// creatures a blight KChoose (1/1) is posed to the blighting player, no
// counter lands before the answer, and the answered (second) creature takes
// both -1/-1 counters.
func TestBlightTwoCreaturesAsksAndAppliesTheAnswerKernel(t *testing.T) {
	t.Parallel()
	e := fixtureEngine(t, 2)
	you1 := placeSrc(t, e, 0, "Name:Fixture you1\nTypes:Creature\nPT:3/3\nOracle:x\n", state.ZBattlefield)
	you2 := placeSrc(t, e, 0, "Name:Fixture you2\nTypes:Creature\nPT:3/3\nOracle:x\n", state.ZBattlefield)
	start := len(e.L.Events)
	counters := func() map[state.ObjID]int32 {
		out := map[state.ObjID]int32{}
		for _, ev := range eventsSince(e, start) {
			if ev.Kind == events.CounterChange && ev.Counter == "M1M1" {
				out[ev.Obj] += ev.Amount
			}
		}
		return out
	}
	d := runProbeSA(t, e, parseFixtureSA(t, "DB$ Blight | Defined$ You | Num$ 2"),
		func() *effects.Ctx { return &effects.Ctx{Source: you1, Controller: 0} }, nil)
	if d == nil || d.ResumeKind != "blight" || d.Player != 0 || d.Min != 1 || d.Max != 1 || len(d.Options) != 2 {
		t.Fatalf("decision = %+v, want seat 0's 1/1 blight KChoose over both creatures", d)
	}
	if got := counters(); len(got) != 0 {
		t.Fatalf("counters %v placed before the answer", got)
	}
	answerAsk(t, e, optionIdxForObj(t, d, you2))
	if got := counters(); got[you2] != 2 || got[you1] != 0 {
		t.Fatalf("counters = %v, want 2 on you2 (%d) and none on you1", got, you2)
	}
}
