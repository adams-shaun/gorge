package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	castSupplySeven = "Name:Seven Probe\nManaCost:7\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	castSupplyFour  = "Name:Four Probe\nManaCost:4\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	castSupplyRock  = "Name:Twin Rock\nManaCost:1\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Amount$ 2 | Produced$ C | SpellDescription$ Two colorless for the probe.\nOracle:x\n"
	// castSupplyAltar is a repeatable non-{T} mana ability (Ashnod's Altar):
	// a tapped source can still sacrifice a creature for two mana.
	castSupplyAltar  = "Name:Supply Altar\nManaCost:0\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ Two for the probe.\nOracle:x\n"
	castSupplyFodder = "Name:Supply Fodder\nManaCost:0\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
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
// by any census-gap prefix (CR 107.5), so potentialCensusFallback skips its
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

// TestCensusTapSupplyRefusesATappedNonTapSource: a mana ability with no {T}
// in its cost (Diamond Kaleidoscope's "Sacrifice a Prism token: Add one mana
// of any color", Ashnod's Altar's) stays activatable while its source is
// tapped, and the sacrifice can be repeated for each token or creature. A
// tapped source is therefore not zero: the bound must refuse rather than skip
// it, or a cast the prefix search can pay (TestReproManifest7's decision 335)
// is pruned.
func TestCensusTapSupplyRefusesATappedNonTapSource(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9918, ppFiller)
	altar := onBoard(t, e, 0, castSupplyAltar)
	onBoard(t, e, 0, castSupplyFodder)
	e.G.Obj(altar).Tapped = true
	mas := e.appendAvailableManaAbilitiesGate(nil, nil, 0, altar, true)
	if len(mas) == 0 {
		t.Fatal("precondition: the tapped altar has no activatable mana ability")
	}
	if e.manaAbilityCC(0, altar, mas[0]).Cost.Tap {
		t.Fatal("precondition: the altar's mana ability costs {T}; the test needs a non-{T} one")
	}
	if supply, ok := censusTapSupply(e, 0); ok {
		t.Fatalf("censusTapSupply = %d, true; a tapped source's non-{T} mana ability must claim no bound", supply)
	}
}

// TestCastOutrunsSupplyIgnoresATappedNonTapSource is the decision-335 shape:
// two Swamps (gaps under Manabarbs) plus a tapped sacrifice source, so the
// four-mana probe is payable only through the tapped source's non-{T} mana
// ability. The old bound skipped the tapped source, counted two mana and
// pruned the cast.
func TestCastOutrunsSupplyIgnoresATappedNonTapSource(t *testing.T) {
	t.Parallel()
	e, _, probe := newFixtureDeck(t, 9919, castSupplyFour)
	for range 2 {
		onBoard(t, e, 0, interferenceSwamp)
	}
	altar := onBoard(t, e, 0, castSupplyAltar)
	onBoard(t, e, 0, castSupplyFodder)
	e.G.Obj(altar).Tapped = true
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	hyp := e.G.Players[0].Pool
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete {
		t.Fatal("precondition: the census is complete; the Swamps under Manabarbs must be gaps")
	}
	if census.outruns != nil && census.outruns(decision.Option{Kind: optCast, Obj: probe}) {
		t.Fatal("the {4} cast over two Swamps and a tapped sacrifice source was proven unpayable")
	}
}
