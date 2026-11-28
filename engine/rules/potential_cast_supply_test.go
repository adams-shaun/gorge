package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	castSupplySeven = "Name:Seven Probe\nManaCost:7\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	castSupplyFour  = "Name:Four Probe\nManaCost:4\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	castSupplyRock  = "Name:Twin Rock\nManaCost:1\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Add {C}{C}.\nOracle:x\n"
)

// castSupplyBoard is seat 0 with three Swamps and a {T}: Add {C}{C} rock
// under seat 1's Manabarbs: every land is a census gap (its tap triggers
// Manabarbs), and the rock makes two mana, so the one-mana-per-source supply
// count (manaTapSingle's plain board) is no bound and the board's best is
// 3 + 2 = 5 mana.
func castSupplyBoard(t *testing.T, spellSrc string) (*Engine, state.ObjID, paymentPlanCensus) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, 9917, spellSrc)
	for range 3 {
		onBoard(t, e, 0, interferenceSwamp)
	}
	onBoard(t, e, 0, castSupplyRock)
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	hyp := e.G.Players[0].Pool
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || len(census.gaps) != 3 {
		t.Fatalf("census complete=%v gaps=%v, want the three Swamps as gaps", census.complete, census.gaps)
	}
	if supply, ok := censusTapSupply(e, 0); !ok || supply != 5 {
		t.Fatalf("tap supply = %d (ok %v), want 5", supply, ok)
	}
	return e, spell, census
}

// TestCastOutrunsSupplySkipsAnUnpayablePlainCast: a {7} plain cast on a
// board whose every source taps once for at most 5 mana is proven unpayable
// by any census-gap prefix (CR 602.2b), so potentialCensusFallback skips its
// prefix search; a {4} cast the same board can pay is not.
func TestCastOutrunsSupplySkipsAnUnpayablePlainCast(t *testing.T) {
	t.Parallel()
	_, seven, census := castSupplyBoard(t, castSupplySeven)
	if census.outruns == nil || !census.outruns(decision.Option{Kind: optCast, Obj: seven}) {
		t.Fatal("the {7} cast over a five-mana board is not proven to outrun its supply")
	}
	_, four, census := castSupplyBoard(t, castSupplyFour)
	if census.outruns != nil && census.outruns(decision.Option{Kind: optCast, Obj: four}) {
		t.Fatal("the {4} cast over a five-mana board was proven unpayable")
	}
}
