package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCensusFixedProductionFullCounts pins the unless-cost census's
// price for a FIXED multi-colour mana production: the activation produces
// every slot of its Produced$ at once, so the fee arm prices it as ONE
// alternative over the full counts. Pinning it per colour (the shape
// pinnedManaAlts serves choice-shaped productions) would price Sunscorched
// Divide's "{1}, {T}: Add {R}{W}" at one unit, and an unless tax the window
// can fund (an Island's {U} pays the {1} fee, CR 605.3a/605.3b) would reach
// the payer as decline-only (Rune Snag, tail-01 pair 143).
func TestUnlessCensusFixedProductionFullCounts(t *testing.T) {
	t.Parallel()
	isle := "Name:Fee Isle\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n"
	divide := "Name:Rift Divide\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ R W\nOracle:x\n"

	// Precondition: the census lists the divide as the payer's fee-carrying
	// source, so the payable assertions below really exercise the fee arm.
	ePre := handEngine(t)
	ePre.G.Players[0].Pool = state.Mana{}
	islePre := onBoard(t, ePre, 0, isle)
	divPre := onBoard(t, ePre, 0, divide)
	feeSource := false
	for _, u := range ePre.unlessWindowUnits(0) {
		for _, a := range u.Alts {
			if u.ID == divPre && a.CostGeneric > 0 {
				feeSource = true
				if got := a.Mana().Total(); got != 2 {
					t.Fatalf("precondition: the fixed {R}{W} production priced as %d units, want 2 (one alternative over the full counts)", got)
				}
			}
		}
	}
	if !feeSource {
		t.Fatal("precondition: the unless census listed no fee-carrying fixed production — premise false")
	}
	if isle := ePre.G.Obj(islePre); isle == nil || isle.Tapped {
		t.Fatal("precondition: the Island not untapped on the battlefield")
	}

	// An empty pool: the Isle's {U} funds the divide's {1} fee (CR 605.3b)
	// and the settle floats {R}{W}, so a {2} tax is payable and a {4} is not.
	e := handEngine(t)
	e.G.Players[0].Pool = state.Mana{}
	onBoard(t, e, 0, isle)
	onBoard(t, e, 0, divide)
	if !e.UnlessCostPayable(0, "2") {
		t.Fatal("the Isle's {U} funds the {1} fee and the settle floats {R}{W}: a {2} tax is payable (CR 605.3a, 605.3b)")
	}
	if e.UnlessCostPayable(0, "4") {
		t.Fatal("the chain floats {U} into {R}{W}, three units: a {4} tax is not payable")
	}
}
