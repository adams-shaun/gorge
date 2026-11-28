package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	reachTen    = "Name:Reach Ten\nManaCost:10\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	reachFive   = "Name:Reach Five\nManaCost:5\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	reachAssist = "Name:Reach Assist\nManaCost:4 B\nTypes:Sorcery\nK:Assist\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	reachTaxer  = "Name:Reach Taxer\nManaCost:1\nTypes:Creature Cleric\nPT:1/1\nS:Mode$ RaiseCost | ValidCard$ Card.nonCreature | Type$ Spell | Amount$ 1 | Description$ Noncreature spells cost one more.\nOracle:x\n"
	// reachTower is Phyrexian Tower's shape: a {T} mana ability whose
	// sacrifice can remove a RaiseCost source mid-prefix.
	reachTower = "Name:Reach Tower\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T Sac<1/Creature> | Produced$ B | Amount$ 2 | SpellDescription$ Two black.\nOracle:x\n"
)

// reachOutruns is the census's cast bound for spell at seat 0's priority.
func reachOutruns(t *testing.T, e *Engine, spell state.ObjID) (supply int32, outruns bool) {
	t.Helper()
	toMain1(t, e)
	e.pending = nil
	e.askPriority(0)
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	supply, ok := censusTapSupply(e, 0)
	if !ok || census.complete {
		t.Fatalf("tap supply ok=%v census complete=%v, want a bounded census with gaps", ok, census.complete)
	}
	return supply, census.outruns != nil && census.outruns(decision.Option{Kind: optCast, Obj: spell})
}

// TestCastOutrunsSupplyIgnoresASelfOnlyStaticElsewhere: Ancient Stone Idol in
// hand prints a self-only ReduceCost (EffectZone$ All). It cannot touch a
// {10} sorcery's price (CR 601.2f), so the bound still proves the sorcery
// unpayable on three Swamps under Manabarbs; the Idol's own cast, which the
// reduction does reach, is not proven.
func TestCastOutrunsSupplyIgnoresASelfOnlyStaticElsewhere(t *testing.T) {
	t.Parallel()
	e, _, ten := newFixtureDeck(t, 9963, reachTen)
	for range 3 {
		onBoard(t, e, 0, interferenceSwamp)
	}
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	idol := onBoardCard(t, e, 0, corpusCard(t, "Ancient Stone Idol"))
	e.emit(events.Event{Kind: events.MoveZone, Obj: idol, From: state.ZBattlefield, To: state.ZHand})
	if e.G.Obj(idol).Zone != state.ZHand || len(e.collectCostStatics().reduce) == 0 {
		t.Fatal("precondition: the Idol's self-only reduction is not collected from the hand")
	}
	supply, outruns := reachOutruns(t, e, ten)
	if supply >= 10 || !outruns {
		t.Fatalf("supply=%d outruns=%v: the {10} sorcery is not proven to outrun its supply", supply, outruns)
	}
	if _, outruns := reachOutruns(t, e, idol); outruns {
		t.Fatal("the Idol's own cast, which its reduction reaches, was proven unpayable")
	}
}

// TestCastOutrunsSupplyRefusesAnAssistCarrier: CR 702.132a lets another seat
// pay the generic part, so a {4}{B} Assist sorcery over a three-mana supply is
// not proven unpayable.
func TestCastOutrunsSupplyRefusesAnAssistCarrier(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9961, reachAssist)
	for range 2 {
		onBoard(t, e, 0, chainDual)
	}
	onBoard(t, e, 0, chainFilter)
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	for range 5 {
		onBoard(t, e, 1, interferenceSwamp)
	}
	supply, outruns := reachOutruns(t, e, spell)
	if supply >= 5 {
		t.Fatalf("precondition: supply=%d is not below the cast's 5", supply)
	}
	if outruns {
		t.Fatal("the Assist cast another seat can fund was proven unpayable")
	}
}

// TestCastOutrunsSupplyRefusesARemovableRaise: a {5} sorcery taxed to 6 by a
// creature's RaiseCost over a five-mana supply is payable once a {T}, Sacrifice
// a creature mana ability in the prefix sacrifices the taxer (CR 601.2f), so
// it is not proven unpayable.
func TestCastOutrunsSupplyRefusesARemovableRaise(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9962, reachFive)
	for range 2 {
		onBoard(t, e, 0, chainDual)
	}
	onBoard(t, e, 0, chainFilter)
	onBoard(t, e, 0, reachTower)
	onBoard(t, e, 0, reachTaxer)
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	if len(e.collectCostStatics().raise) == 0 {
		t.Fatal("precondition: the taxer's RaiseCost is not collected")
	}
	supply, outruns := reachOutruns(t, e, spell)
	if supply < 5 || supply >= 6 {
		t.Fatalf("precondition: supply=%d, want 5 (pays the untaxed cost, not the taxed one)", supply)
	}
	if outruns {
		t.Fatal("the cast a prefix can untax was proven unpayable")
	}
}
