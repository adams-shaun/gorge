package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	// chainLotus taps to choose a colour and add that much devotion: its whole
	// production sits in the SubAbility$, none on the head (Nyx Lotus).
	chainLotus = "Name:Chain Lotus\nManaCost:4\nTypes:Artifact\n" +
		"A:AB$ ChooseColor | Cost$ T | Defined$ You | SubAbility$ DBMana | SpellDescription$ Choose a colour, then add that devotion.\n" +
		"SVar:DBMana:DB$ Mana | Produced$ Chosen | Amount$ X\nSVar:X:Count$Devotion.Chosen\nOracle:x\n"
	chainDevotee = "Name:Chain Devotee\nManaCost:B B B\nTypes:Creature Cleric\nPT:1/1\nOracle:x\n"
	// chainDual has two mana abilities, so the potential walk's per-ability
	// bound counts it twice while it taps once.
	chainDual = "Name:Chain Dual\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Add black.\n" +
		"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add red.\nOracle:x\n"
	chainFive = "Name:Chain Five\nManaCost:5\nTypes:Sorcery\nA:SP$ Draw | NumCards$ 1\nOracle:x\n"
	// chainGuidance adds one green per Elf when the enchanted land taps
	// (Elvish Guidance): a triggered mana ability whose amount the tap-extra
	// bound does not count.
	chainGuidance = "Name:Chain Guidance\nManaCost:2 G\nTypes:Enchantment Aura\nK:Enchant:Land\n" +
		"T:Mode$ TapsForMana | ValidCard$ Card.AttachedBy | Execute$ ChainElfAdd | Static$ True | TriggerDescription$ Add one green per Elf.\n" +
		"SVar:ChainElfAdd:DB$ Mana | Produced$ G | Amount$ X | Defined$ TriggeredCardController\n" +
		"SVar:X:Count$Valid Creature.Elf\nOracle:x\n"
	chainElf    = "Name:Chain Elf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	chainFilter = "Name:Chain Filter\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ BR T | Produced$ Combo B R | Amount$ 2 | SpellDescription$ Filter for two.\nOracle:x\n"
)

// chainPlanOffersCast asserts that the census does not claim a supply bound
// proving the cast unpayable and that the plan walk answers it with a prefix
// script (the payable play is offered).
func chainPlanOffersCast(t *testing.T, e *Engine, spell, gap state.ObjID) {
	t.Helper()
	hyp := e.PotentialMana(0)
	census := e.paymentPlanCensusOf(0, &hyp)
	if census.complete || !slices.Contains(census.gaps, gap) {
		t.Fatalf("precondition: census complete=%v gaps=%v, want source %d as a gap", census.complete, census.gaps, gap)
	}
	if supply, ok := censusTapSupply(e, 0); ok {
		t.Errorf("censusTapSupply = %d, true; a source making more than the bound reads must claim no bound", supply)
	}
	for _, pp := range e.potentialPlans(0, potentialScriptPlays) {
		if pp.Action.Kind != optCast || pp.Action.Obj != spell {
			continue
		}
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("payable cast judged reason=%q detail=%q script=%v, want a prefix script", pp.Reason, pp.Detail, pp.Script)
		}
		return
	}
	t.Fatal("precondition: the potential walk lists no cast of the spell")
}

// CR 605.1a puts no order on a mana ability's effects: Chain Lotus's one tap
// makes nine black (devotion, CR 700.5) although its head names no mana, so a
// supply count reading the head alone proved a payable five-mana cast
// unpayable.
func TestCensusTapSupplyRefusesAChainedManaProduction(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9931, chainFive)
	lotus := onBoard(t, e, 0, chainLotus)
	for range 3 {
		onBoard(t, e, 0, chainDevotee)
	}
	for range 2 {
		onBoard(t, e, 0, chainDual)
	}
	toMain1(t, e)
	e.pending = nil
	e.askPriority(0)
	chainPlanOffersCast(t, e, spell, lotus)
}

// CR 605.1b: a triggered mana ability adds its mana with the tap. Chain
// Guidance's counted amount is not part of the tap-extra bound, so a supply
// count over that bound must be refused rather than prune the cast the
// enchanted land's tap pays.
func TestCensusTapSupplyRefusesAnUncountedTapTrigger(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9932, chainFive)
	for range 3 {
		onBoard(t, e, 0, chainElf)
	}
	for range 2 {
		onBoard(t, e, 0, chainDual)
	}
	// A hybrid-fee filter under Manabarbs is a census gap with no relaxed
	// alternative, so neither relaxed proof runs and only the supply count
	// could skip the prefix search.
	filter := onBoard(t, e, 0, chainFilter)
	onBoardCard(t, e, 1, corpusCard(t, "Manabarbs"))
	land := onBoard(t, e, 0, ssForest)
	aura := onBoard(t, e, 0, chainGuidance)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{land}})
	toMain1(t, e)
	e.pending = nil
	e.askPriority(0)
	if got := e.G.Obj(aura).AttachedTo; got != land {
		t.Fatalf("precondition: Chain Guidance attached to %d, want %d", got, land)
	}
	hyp := e.PotentialMana(0)
	if census := e.paymentPlanCensusOf(0, &hyp); census.relaxable || census.bounded {
		t.Fatalf("precondition: census relaxable=%v bounded=%v, want neither relaxed proof to apply", census.relaxable, census.bounded)
	}
	chainPlanOffersCast(t, e, spell, filter)
}

// A tap trigger no permanent prints -- one a static grants, or one an effect
// registered (High Tide) -- is a triggered mana ability the tap-extra bound
// does not count either, so both refuse the supply bound; the same board
// without it keeps the bound.
func TestCensusTapSupplyRefusesAGrantedOrRegisteredTapTrigger(t *testing.T) {
	t.Parallel()
	const granter = "Name:Chain Granter\nManaCost:1 G\nTypes:Enchantment\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddTrigger$ TrigTap | Description$ Lands add one more.\n" +
		"SVar:TrigTap:Mode$ TapsForMana | ValidCard$ Card.Self | Execute$ ChainGrantAdd | Static$ True | TriggerDescription$ Add one green.\n" +
		"SVar:ChainGrantAdd:DB$ Mana | Produced$ G\nOracle:x\n"
	for _, tc := range []struct {
		name string
		add  func(t *testing.T, e *Engine)
	}{
		{"granted", func(t *testing.T, e *Engine) {
			onBoard(t, e, 0, granter)
			if !slices.ContainsFunc(e.active(), func(ce ContinuousEffect) bool { return ce.AddTrigger != nil }) {
				t.Fatal("precondition: no continuous effect grants a trigger")
			}
		}},
		{"registered", func(_ *testing.T, e *Engine) {
			e.G.Delayed = append(e.G.Delayed, state.DelayedTrigger{EventMode: "TapsForMana", EffectRepeat: true})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, _, _ := newFixtureDeck(t, 9933, chainFive)
			onBoard(t, e, 0, ssForest)
			if supply, ok := censusTapSupply(e, 0); !ok || supply != 1 {
				t.Fatalf("precondition: the lone Forest's tap supply = %d (ok %v), want 1", supply, ok)
			}
			tc.add(t, e)
			if supply, ok := censusTapSupply(e, 0); ok {
				t.Fatalf("censusTapSupply = %d, true beside a %s tap trigger", supply, tc.name)
			}
		})
	}
}
