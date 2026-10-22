package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCyclingPosesNoSelfDiscardAsk pins CR 602.2b for Discard<1/CARDNAME>
// costs: paying "discard this card" has exactly one possible answer, so
// activating cycling poses no KChoose; the card goes to the graveyard and the
// ability is on the stack.
func TestCyclingPosesNoSelfDiscardAsk(t *testing.T) {
	t.Parallel()
	const cycler = "Name:Plain Cycler\nManaCost:3 U\nTypes:Creature Beast\nPT:2/2\nK:Cycling:1\nOracle:x\n"
	e := handEngine(t, card(t, cycler))
	id := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand || !o.Face().HasKeyword("Cycling") {
		t.Fatalf("precondition: cycler must be in hand and print Cycling: %+v", o)
	}
	addMana(t, e, 0, "C")
	var opt decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id && strings.Contains(o.Cost, "Discard<1/CARDNAME>") {
			opt, found = o, true
		}
	}
	if !found {
		t.Fatal("precondition: cycling activation (Discard<1/CARDNAME> cost) not offered")
	}
	e.beginActivation(0, opt)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("self-discard cost posed a KChoose: %+v", d)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("cycled card in %s, want graveyard", got)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack depth = %d, want the cycling ability alone", len(e.G.Stack))
	}
	if o := e.G.Obj(e.G.Stack[0]); o == nil || o.Ability == nil {
		t.Fatalf("stack top is not an ability: %+v", o)
	}
}
