package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// feeOnlyClassBoard is the class board of the pinned Orzhova game: the play's
// source, the fee-only repeatable source (Energy Refractor's "{2}: Add one
// mana of any color") and enough lands to pay {3}{W}{B} only by funding the
// Refractor's fee from two of them first (CR 602.2b).
func feeOnlyClassBoard(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := boundBoard(t, []string{"Mountain", "Mountain", "Plains", "Forest", "Forest", "Forest", "Orzhova, the Church of Deals", "Energy Refractor"}, nil)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Orzhova, the Church of Deals" {
			return e, id
		}
	}
	t.Fatal("Orzhova is not on the battlefield")
	return nil, 0
}

// The plain prefix run finds the funding line within potentialScriptNodes
// once the fee-only gap's colour ask drops the answers that fill no pip the
// play lacks (feeOnlyAskPrune); unpruned it spends the budget on the
// Refractor's own five-way colour branches.
func TestFeeOnlyAskPruneFindsTheFundingLine(t *testing.T) {
	t.Parallel()
	e, orz := feeOnlyClassBoard(t)
	_, opts := e.potentialWalkOf(0, true)
	var play = -1
	for i, o := range opts {
		if o.Kind == "ability" && o.Obj == orz {
			play = i
		}
	}
	if play < 0 {
		t.Fatal("precondition: the potential walk has no Orzhova activation option")
	}
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) == 0 {
		t.Fatal("precondition: the census must leave the fee-only Refractor as a gap")
	}
	script, ok := e.potentialPrefixScript(0, opts[play], census.gaps, census.fees())
	if !ok || len(script) == 0 {
		t.Fatalf("the prefix script paying Orzhova's activation through the Refractor was not found (ok=%v)", ok)
	}
}
