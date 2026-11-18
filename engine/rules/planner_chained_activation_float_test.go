package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	pcfPlain = "Name:Chained Float Plain\nManaCost:W W\nTypes:Sorcery\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\nOracle:x\n"
	// pcfDiscardX is Vengeful Dreams' shape: an announced Discard<X> beside the
	// printed mana, X = 0 castable.
	pcfDiscardX = "Name:Chained Float Discard\nManaCost:W W\nTypes:Instant\n" +
		"A:SP$ ChangeZone | Cost$ W W Discard<X/Card/card(s)> | CostDesc$ As an additional cost to cast this spell, discard X cards. | Origin$ Battlefield | Destination$ Exile | TargetMin$ X | TargetMax$ X | ValidTgts$ Creature.attacking | TgtPrompt$ Select X target attacking creatures | SpellDescription$ Exile X target attacking creatures.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	pcfIsland  = "Name:Chained Float Island\nManaCost:no cost\nTypes:Basic Land Island\nOracle:x\n"
	pcfThicket = "Name:Chained Float Thicket\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Produced$ Combo W G | Cost$ T\nOracle:x\n"
	pcfSignet = "Name:Chained Float Signet\nManaCost:2\nTypes:Artifact\n" +
		"A:AB$ Mana | Produced$ W B | Cost$ 1 T\nOracle:x\n"
)

// CR 601.2g: mana abilities are activated in any order while paying, so one
// source's float pays another's activation cost. {W}{W} is payable from an
// Island ({T}: {U}), a {G}/{W} land and a {1},{T}: {W}{B} signet: Island's {U}
// pays the signet's {1}, the signet makes {W}{B}, the land adds the second {W}.
// The planner's composition cannot chain the signet's fee onto another
// source's float (planComposedCost), so the chained activations are a prefix
// script (potentialPrefixScript): a spell whose extras are an announced
// Discard<X> (X = 0 pays nothing extra) reaches it like any plain cast.
func TestPlannerChainedActivationFloat(t *testing.T) {
	t.Parallel()
	for _, src := range []string{pcfPlain, pcfDiscardX} {
		e, _, spell := newFixtureDeck(t, 9412, src)
		onBoard(t, e, 0, pcfIsland)
		onBoard(t, e, 0, pcfThicket)
		onBoard(t, e, 0, pcfSignet)
		e.G.Step = state.StepMain1
		ppPriority(t, e)
		// precondition: the composition alone prices the cast insufficient
		// (the chain is beyond it), so the verdict below is the script's.
		if got := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: spell}); got.Reason != "insufficient" {
			t.Fatalf("%.28s: composition = %q, want insufficient (the chain is not in it)", src, got.Reason)
		}
		var found bool
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind != "cast" || pp.Action.Obj != spell {
				continue
			}
			found = true
			if pp.Reason != "" || pp.Detail != "" {
				t.Fatalf("%.28s: cast priced %q/%q, want payable", src, pp.Reason, pp.Detail)
			}
			if len(pp.Script) < 2 {
				t.Fatalf("%.28s: script = %d steps, want the island then the signet (the thicket pays the rest)", src, len(pp.Script))
			}
		}
		if !found {
			t.Fatalf("%.28s: no cast plan for the spell", src)
		}
	}
}
