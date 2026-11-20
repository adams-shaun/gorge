package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSkipPhaseShapesCompile pins that the 14 corpus cards carrying a modelled
// SkipPhase shape (no "unimplemented SkipPhase shape" Note) compile to the step and amount their text says.
func TestSkipPhaseShapesCompile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	type shape struct {
		step   state.Step
		amount int32
	}
	want := map[string]shape{
		"Avizoa":                     {state.StepUntap, 0},
		"Brine Elemental":            {state.StepUntap, 0},
		"Dovin, Architect of Law":    {state.StepUntap, 0},
		"Shisato, Whispering Hunter": {state.StepUntap, 0},
		"Elfhame Sanctuary":          {state.StepDraw, 0},
		"Fatigue":                    {state.StepDraw, 0},
		"Ivory Gargoyle":             {state.StepDraw, 0},
		"Molten Firebird":            {state.StepDraw, 0},
		"Blinding Angel":             {state.StepBeginCombat, 1},
		"Revenant Patriarch":         {state.StepBeginCombat, 1},
		"Stonehorn Dignitary":        {state.StepBeginCombat, 1},
		"Moment of Silence":          {state.StepBeginCombat, 2},
		"False Peace":                {state.StepBeginCombat, 0},
		"Empty City Ruse":            {state.StepBeginCombat, 0},
	}
	got := map[string]int{}
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			visit := func(sa *cards.SA) {
				for ; sa != nil; sa = sa.Sub {
					if sa.API != "SkipPhase" {
						continue
					}
					w, ok := want[face.Name]
					if !ok {
						continue // a shape outside the 14 (Fatespinner) stays a Note
					}
					p := compileSkipPhaseParams(sa)
					if !p.ok {
						t.Errorf("%s: unmodelled SkipPhase shape: %s", face.Name, p.desc)
						continue
					}
					got[face.Name]++
					if p.step != w.step || p.amount != w.amount {
						t.Errorf("%s: step %v amount %d, want step %v amount %d", face.Name, p.step, p.amount, w.step, w.amount)
					}
				}
			}
			visit(face.SpellAbility())
			face.EachSVarAbility(visit)
		}
	}
	for name := range want {
		if got[name] == 0 {
			t.Errorf("%s: no SkipPhase SA found", name)
		}
	}
}
