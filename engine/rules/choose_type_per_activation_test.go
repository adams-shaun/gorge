package rules

// CR 608.2: each resolution of an activated ability makes its own choice. A
// permanent whose AB$ ChooseType recorded a type on an earlier activation
// (Dream Thrush's "the basic land type of your choice") asks again on the
// next one, and the new answer replaces the old; the recorded type is not an
// as-enters choice the ability reuses.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const chooseTypeActivationProbe = "Name:Choose Probe\nTypes:Artifact\n" +
	"A:AB$ ChooseType | Cost$ 1 | Defined$ You | Type$ Basic Land | SpellDescription$ x\nOracle:x\n"

// chooseTypeAskOf returns the KChoose type ask d (or a later pending one) is,
// answering any payment decision in between with its first option.
func chooseTypeAskOf(t *testing.T, e *Engine, d *decision.Decision) *decision.Decision {
	t.Helper()
	for i := 0; d != nil && i < 8; i++ {
		if d.Kind == decision.KChoose && d.ResumeKind == "choosetype" {
			return d
		}
		submitChoices(t, e, 0)
		d = nextAfterSettle(t, e)
	}
	return nil
}

func TestChooseTypeAsksOnEveryActivation(t *testing.T) {
	t.Parallel()
	e := fixtureEngine(t, 2)
	src := placeSrc(t, e, 0, chooseTypeActivationProbe, state.ZBattlefield)
	d := chooseTypeAskOf(t, e, activatePermanentAbility(t, e, 0, src))
	if d == nil {
		t.Fatal("the first activation posed no type ask")
	}
	submitChoices(t, e, optionIdxWithLabel(t, d, "Mountain"))
	nextAfterSettle(t, e)
	if got := e.G.Obj(src).ChosenType; got != "Mountain" {
		t.Fatalf("after the first activation ChosenType = %q, want Mountain", got)
	}
	d = chooseTypeAskOf(t, e, activatePermanentAbility(t, e, 0, src))
	if d == nil {
		t.Fatal("CR 608.2: the second activation reused the first answer instead of asking")
	}
	submitChoices(t, e, optionIdxWithLabel(t, d, "Swamp"))
	nextAfterSettle(t, e)
	if got := e.G.Obj(src).ChosenType; got != "Swamp" {
		t.Fatalf("after the second activation ChosenType = %q, want the new answer Swamp", got)
	}
}
