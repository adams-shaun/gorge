package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCostPayableHybridFeeFilter: a filter land whose activation fee is a
// hybrid pip ("{B/R}, {T}: Add {B}{B}, {B}{R} or {R}{R}", Graven Cairns) joins the
// unless census (CR 107.4b: the payer announces the pip's colour, CR 605.3b: its
// fee is paid from mana an earlier activation made). Three Mountains and the
// land make {R}{R}{R}{R}: one Mountain pays the pip, the Cairns makes {R}{R}, the
// other two Mountains {R}{R}. The census dropped the hybrid fee, so gorge offered
// decline only where the kernel offered pay (Dragon Tyrant's upkeep).
func TestUnlessCostPayableHybridFeeFilter(t *testing.T) {
	t.Parallel()
	mountain := "Name:Fee Mountain\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n"
	filter := "Name:Fee Cairns\nTypes:Land\n" +
		"A:AB$ Mana | Produced$ C | Cost$ T | SpellDescription$ Add {C}.\n" +
		"A:AB$ Mana | Cost$ BR T | Produced$ Combo B R | Amount$ 2 | SpellDescription$ Add two.\nOracle:x\n"
	build := func(mountains int) *Engine {
		e := handEngine(t)
		e.G.Players[0].Pool = state.Mana{}
		for range mountains {
			onBoard(t, e, 0, mountain)
		}
		onBoard(t, e, 0, filter)
		return e
	}
	e := build(3)
	feeListed := false
	for _, u := range e.unlessWindowUnits(0) {
		for _, a := range u.Alts {
			if a.CostColored.Total() > 0 {
				feeListed = true
			}
		}
	}
	if !feeListed {
		t.Fatal("precondition: the unless census listed no hybrid-fee alt for the filter land")
	}
	if !e.UnlessCostPayable(0, "R R R R") {
		t.Fatal("three Mountains and a {B/R} filter land make {R}{R}{R}{R}; the tax is payable")
	}
	if build(2).UnlessCostPayable(0, "R R R R") {
		t.Fatal("two Mountains and the filter land make only {R}{R}{R}; {R}{R}{R}{R} is not payable")
	}
}
