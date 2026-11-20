package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSkipPhaseStepShapesLower: the DB$ SkipPhase shapes gorge's
// compileSkipPhaseParams models beyond False Peace's lower to ExtSkipPhase with
// the matching kop.SkipPhase* grant and no refusal (the card's other lines are
// not under test: Yosei's Tap filter is a different refusal).
func TestSkipPhaseStepShapesLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	want := map[string]int{
		"Avizoa":                     skipC(kop.StepUntap, skipScopeNext),
		"Brine Elemental":            skipC(kop.StepUntap, skipScopeNext),
		"Shisato, Whispering Hunter": skipC(kop.StepUntap, skipScopeNext),
		"Fatigue":                    skipC(kop.StepDraw, skipScopeNext),
		"Ivory Gargoyle":             skipC(kop.StepDraw, skipScopeNext),
		"Molten Firebird":            skipC(kop.StepDraw, skipScopeNext),
		"Elfhame Sanctuary":          skipC(kop.StepDraw, skipScopeTurn),
		"Blinding Angel":             skipC(kop.StepBeginCombat, skipScopeNext),
		"Revenant Patriarch":         skipC(kop.StepBeginCombat, skipScopeNext),
		"Stonehorn Dignitary":        skipC(kop.StepBeginCombat, skipScopeNext),
		"Moment of Silence":          skipC(kop.StepBeginCombat, skipScopeTurn),
		"False Peace":                0,
	}
	for name, grant := range want {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		lc := lowerCard(p, card.Faces[0].Name, card, false)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, lc.Refusals)
		}
		got, found := -1, false
		for _, ab := range lc.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "SkipPhase" {
					got, found = in.C, true
				}
			}
		}
		if !found || got != grant {
			t.Errorf("%s: SkipPhase grant = %d (found %v), want %d", name, got, found, grant)
		}
	}
}
