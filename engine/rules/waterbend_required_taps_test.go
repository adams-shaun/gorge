package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 701.67a: an announcement that taps too few helpers leaves a waterbend cost
// the mana window can't finish. Yue's Waterbend {5} with three Islands needs both
// Drones; with five Islands it needs neither.
func TestWaterbendAskRequiresTheTapsThePaymentNeeds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		islands, min int
	}{{3, 2}, {4, 1}, {5, 0}} {
		e, _, _ := newFixtureDeck(t, 9700, ppFiller)
		toMain1(t, e)
		src := onBoardCard(t, e, 0, corpusCard(t, "Yue, the Moon Spirit"))
		e.G.Obj(src).SummonSick = false
		for range 2 {
			e.G.Obj(onBoard(t, e, 0, apaDrone)).SummonSick = false
		}
		for range tc.islands {
			e.G.Obj(onBoard(t, e, 0, apaIsland)).SummonSick = false
		}
		d := ppPriority(t, e)
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == src {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%d Islands: the ability is not offered: %+v", tc.islands, d.Options)
		}
		submitChoices(t, e, idx)
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose || ask.Source != src {
			t.Fatalf("%d Islands: pending %+v, want the helper ask", tc.islands, ask)
		}
		if ask.Min != tc.min || ask.Max != 2 {
			t.Errorf("%d Islands: the helper ask takes %d to %d, want %d to 2", tc.islands, ask.Min, ask.Max, tc.min)
		}
	}
}

// An activation whose cost has {T} can't also tap its source for mana (CR 602.2b):
// the ask requires the taps that leaves short, and the source taps once.
func TestOwnTapSourcePaysNoMana(t *testing.T) {
	t.Parallel()
	const bender = "Name:Test Bender\nTypes:Creature Elf\nPT:1/1\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Two.\n" +
		"A:AB$ GainLife | Cost$ Waterbend<3> T | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 9700, ppFiller)
	toMain1(t, e)
	src := onBoard(t, e, 0, bender)
	e.G.Obj(src).SummonSick = false
	for range 2 {
		e.G.Obj(onBoard(t, e, 0, apaDrone)).SummonSick = false
	}
	e.G.Obj(onBoard(t, e, 0, apaIsland)).SummonSick = false
	d := ppPriority(t, e)
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == src {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the ability is not offered: %+v", d.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, idx)
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || ask.Min != 2 {
		t.Fatalf("pending %+v, want the helper ask requiring both Drones", ask)
	}
	submitChoices(t, e, 0, 1)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.PaymentFallback != nil || len(e.G.Stack) != 1 {
		t.Fatalf("pending %+v, stack %v: want the ability on the stack, paid", d, e.G.Stack)
	}
	taps := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Tap && ev.Obj == src {
			taps++
		}
	}
	if taps != 1 {
		t.Errorf("the source tapped %d times, want once", taps)
	}
}

// A source that can't help this payment says nothing about the taps it needs: one
// tapped already, or one whose mana this ability can't spend (CR 106.6).
func TestWaterbendAskIgnoresASourceThatCantHelp(t *testing.T) {
	t.Parallel()
	const spellOnly = "Name:Test Scholar\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ Mana | Cost$ T | Produced$ U | RestrictValid$ Spell.Instant | SpellDescription$ Spell mana.\nOracle:x\n"
	for _, tapped := range []bool{false, true} {
		e, _, _ := newFixtureDeck(t, 9700, ppFiller)
		toMain1(t, e)
		src := onBoardCard(t, e, 0, corpusCard(t, "Yue, the Moon Spirit"))
		e.G.Obj(src).SummonSick = false
		for range 2 {
			e.G.Obj(onBoard(t, e, 0, apaDrone)).SummonSick = false
		}
		for range 3 {
			e.G.Obj(onBoard(t, e, 0, apaIsland)).SummonSick = false
		}
		scholar := onBoard(t, e, 0, spellOnly)
		e.G.Obj(scholar).SummonSick = false
		e.G.Obj(scholar).Tapped = tapped
		d := ppPriority(t, e)
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == src {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("tapped %v: the ability is not offered: %+v", tapped, d.Options)
		}
		submitChoices(t, e, idx)
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose || ask.Min != 2 {
			t.Errorf("tapped %v: pending %+v, want a helper ask requiring two", tapped, ask)
		}
	}
}

const (
	testRock = "Name:Test Rock\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless.\nOracle:x\n"
	testBender = "Name:Test Bender\nTypes:Enchantment\n" +
		"A:AB$ GainLife | Cost$ Waterbend<3> | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
)

// A helper that is also a mana source can't do both (CR 601.2g-h): tapping the Rock
// leaves the cost short, an illegal action that is undone before any land taps (CR
// 733.1). The ability stays offered, and tapping the Drone instead pays.
func TestWaterbendHelperThatStrandsThePaymentReverses(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9700, ppFiller)
	toMain1(t, e)
	src := onBoard(t, e, 0, testBender)
	rockID := onBoard(t, e, 0, testRock)
	drone := onBoard(t, e, 0, apaDrone)
	land := onBoard(t, e, 0, apaIsland)
	activate := func() {
		t.Helper()
		idx := -1
		for _, o := range ppPriority(t, e).Options {
			if o.Kind == "ability" && o.Obj == src {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("the ability is not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, idx)
	}
	pick := func(id state.ObjID) {
		t.Helper()
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose || ask.Min != 1 {
			t.Fatalf("pending %+v, want a helper ask requiring one", ask)
		}
		for _, o := range ask.Options {
			if o.Obj == id {
				submitChoices(t, e, o.Index)
				return
			}
		}
		t.Fatalf("%d is not offered: %+v", id, ask.Options)
	}
	activate()
	pick(rockID)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || len(e.G.Stack) != 0 {
		t.Fatalf("pending %+v, stack %v: want the activation reversed", d, e.G.Stack)
	}
	if e.G.Obj(rockID).Tapped || e.G.Obj(land).Tapped {
		t.Errorf("rock tapped %v, land tapped %v: nothing should tap", e.G.Obj(rockID).Tapped, e.G.Obj(land).Tapped)
	}
	activate()
	pick(drone)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.PaymentFallback != nil || len(e.G.Stack) != 1 {
		t.Fatalf("pending %+v, stack %v: want the ability on the stack, paid", d, e.G.Stack)
	}
	if !e.G.Obj(drone).Tapped || !e.G.Obj(land).Tapped || !e.G.Obj(rockID).Tapped {
		t.Errorf("drone, land and rock should all have tapped: %v %v %v", e.G.Obj(drone).Tapped, e.G.Obj(land).Tapped, e.G.Obj(rockID).Tapped)
	}
}

// The same answer on a manual cast, with a Convoke-family keyword: the cast returns to
// hand unless a source the census can't price, or a production something changes,
// might close the gap, and then the window still opens.
func TestConvokeAnswerThatStrandsThePaymentReverses(t *testing.T) {
	t.Parallel()
	const (
		spell = "Name:Test Rite\nManaCost:4\nTypes:Sorcery\nK:Convoke\nA:SP$ GainLife | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
		dork  = "Name:Test Dork\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Green.\nOracle:x\n"
		guide = "Name:Test Guide\nManaCost:2 G\nTypes:Creature Elf\nPT:2/2\n" +
			"A:AB$ Mana | Cost$ ExileFromHand<1/CARDNAME> | ActivationZone$ Hand | Produced$ G | SpellDescription$ Green.\nOracle:x\n"
		doubler = "Name:Test Doubler\nTypes:Enchantment\n" +
			"T:Mode$ TapsForMana | ValidCard$ Land | Static$ True | Execute$ Extra | TriggerDescription$ More.\n" +
			"SVar:Extra:DB$ Mana | Produced$ U | Defined$ TriggeredActivator\nOracle:x\n"
	)
	for _, tc := range []struct {
		name     string
		setup    func(*testing.T, *Engine)
		reversed bool
	}{
		{"nothing else", func(*testing.T, *Engine) {}, true},
		{"a source in hand", func(t *testing.T, e *Engine) { moveSeeded(t, e, 0, guide, state.ZHand) }, false},
		{"a tap trigger", func(t *testing.T, e *Engine) { onBoard(t, e, 0, doubler) }, false},
	} {
		e, _, id := newFixtureDeck(t, 9700, spell, guide)
		toMain1(t, e)
		helper := onBoard(t, e, 0, dork)
		e.G.Obj(helper).SummonSick = false
		for range 3 {
			onBoard(t, e, 0, apaDrone)
		}
		land := onBoard(t, e, 0, apaIsland)
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
		tc.setup(t, e)
		idx := -1
		for _, o := range ppPriority(t, e).Options {
			if o.Kind == "cast" && o.Obj == id {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%s: the spell is not offered: %+v", tc.name, e.Pending().Options)
		}
		submitChoices(t, e, idx)
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose || ask.Source != id {
			t.Fatalf("%s: pending %+v, want the convoke ask", tc.name, ask)
		}
		choice := -1
		for _, o := range ask.Options {
			if o.Kind == "convoke_generic" && o.Obj == helper {
				choice = o.Index
			}
		}
		submitChoices(t, e, choice)
		d := e.Pending()
		if tc.reversed {
			if d == nil || d.Kind != decision.KPriority || e.G.Obj(id).Zone != state.ZHand || e.G.Obj(land).Tapped {
				t.Errorf("%s: pending %+v, spell in %v, land tapped %v: want the cast reversed, nothing tapped", tc.name, d, e.G.Obj(id).Zone, e.G.Obj(land).Tapped)
			}
		} else if d == nil || d.Kind != decision.KChoose || e.G.Obj(id).Zone != state.ZStack {
			t.Errorf("%s: pending %+v, spell in %v: want the mana window", tc.name, d, e.G.Obj(id).Zone)
		}
	}
}

// A permanent elected to a tap cost can't also tap for mana: the same reversal.
func TestTapCostElectionThatStrandsThePaymentReverses(t *testing.T) {
	t.Parallel()
	const (
		garden = "Name:Test Garden\nTypes:Enchantment\n" +
			"A:AB$ GainLife | Cost$ 2 tapXType<1/Creature> | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
		dork = "Name:Test Dork\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Green.\nOracle:x\n"
	)
	for _, tc := range []struct {
		name     string
		pickDork bool
		stack    int
	}{{"the source", true, 0}, {"a creature that makes no mana", false, 1}} {
		e, _, _ := newFixtureDeck(t, 9700, ppFiller)
		toMain1(t, e)
		src := onBoard(t, e, 0, garden)
		source := onBoard(t, e, 0, dork)
		e.G.Obj(source).SummonSick = false
		plain := onBoard(t, e, 0, apaDrone)
		land := onBoard(t, e, 0, apaIsland)
		idx := -1
		for _, o := range ppPriority(t, e).Options {
			if o.Kind == "ability" && o.Obj == src {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%s: the ability is not offered: %+v", tc.name, e.Pending().Options)
		}
		submitChoices(t, e, idx)
		want := plain
		if tc.pickDork {
			want = source
		}
		choice := -1
		for _, o := range e.Pending().Options {
			if o.Obj == want {
				choice = o.Index
			}
		}
		submitChoices(t, e, choice)
		if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.PaymentFallback != nil || len(e.G.Stack) != tc.stack {
			t.Fatalf("%s: pending %+v, stack %v, want priority with %d on the stack", tc.name, d, e.G.Stack, tc.stack)
		}
		if tapped := e.G.Obj(land).Tapped; tapped != !tc.pickDork {
			t.Errorf("%s: land tapped %v", tc.name, tapped)
		}
	}
}

// A mana ability whose condition this build doesn't evaluate adds no mana (CR 106.5),
// so a source that has only that one doesn't leave the census incomplete: the helper
// ask still requires what the payment needs, and the window doesn't offer it.
func TestManaAbilityThatAddsNothingLeavesTheCensusComplete(t *testing.T) {
	t.Parallel()
	const moot = "Name:Test Moot\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo Unwritten | Condition$ While the tide is out. | SpellDescription$ Unmodelled.\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 9700, ppFiller)
	toMain1(t, e)
	src := onBoard(t, e, 0, testBender)
	helpers := []state.ObjID{onBoard(t, e, 0, apaDrone), onBoard(t, e, 0, apaDrone)}
	onBoard(t, e, 0, apaIsland)
	onBoard(t, e, 0, moot)
	idx := -1
	for _, o := range ppPriority(t, e).Options {
		if o.Kind == "ability" && o.Obj == src {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the ability is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || ask.Min != 2 {
		t.Fatalf("pending %+v, want a helper ask requiring both Drones", ask)
	}
	submitChoices(t, e, 0, 1)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.PaymentFallback != nil || len(e.G.Stack) != 1 {
		t.Fatalf("pending %+v, stack %v: want the ability on the stack, paid", d, e.G.Stack)
	}
	for _, id := range helpers {
		if !e.G.Obj(id).Tapped {
			t.Errorf("helper %d is untapped", id)
		}
	}
}

// With nothing else to pay for a raised cost, the target that raises it stays out of
// the menu although the board has a source that adds nothing.
func TestTargetMenuPricesPastASourceThatAddsNothing(t *testing.T) {
	t.Parallel()
	const (
		moot = "Name:Test Moot\nTypes:Land\n" +
			"A:AB$ Mana | Cost$ T | Produced$ Combo Unwritten | Condition$ While the tide is out. | SpellDescription$ Unmodelled.\nOracle:x\n"
		tax = "Name:Test Tax\nTypes:Enchantment\n" +
			"S:Mode$ RaiseCost | ValidCard$ Card.nonCreature | ValidTarget$ Player | Type$ Spell | Amount$ 8 | Description$ Dear.\nOracle:x\n"
		spell = "Name:Test Word\nManaCost:U\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ Word.\nOracle:x\n"
	)
	e, _, id := newFixtureDeck(t, 9700, spell)
	toMain1(t, e)
	onBoard(t, e, 0, apaIsland)
	onBoard(t, e, 0, moot)
	onBoard(t, e, 0, tax)
	onBoard(t, e, 0, apaDrone)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 1})
	idx := -1
	for _, o := range ppPriority(t, e).Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the spell is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KTarget {
		t.Fatalf("pending %+v, want the target ask", ask)
	}
	for _, o := range ask.Options {
		if o.Kind == "player" {
			t.Errorf("offered a target the payment can't reach: %+v", o)
		}
	}
}

// A tap trigger that adds mana is reach the census doesn't price, so it can't withhold
// a target the payment can reach only with that mana (CR 605.1b).
func TestTargetMenuKeepsATargetATapTriggerPaysFor(t *testing.T) {
	t.Parallel()
	const (
		bounty = "Name:Test Bounty\nTypes:Enchantment\n" +
			"T:Mode$ TapsForMana | ValidCard$ Land | Activator$ You | TriggerZones$ Battlefield | Static$ True | Execute$ Extra | TriggerDescription$ More.\n" +
			"SVar:Extra:DB$ Mana | Produced$ U | Amount$ 1\nOracle:x\n"
		tax = "Name:Test Tax\nTypes:Enchantment\n" +
			"S:Mode$ RaiseCost | ValidCard$ Card | ValidTarget$ Player | Type$ Spell | Amount$ 2 | Description$ Dear.\nOracle:x\n"
		spell = "Name:Test Word\nManaCost:2\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ Word.\nOracle:x\n"
	)
	e, _, id := newFixtureDeck(t, 9700, spell)
	toMain1(t, e)
	land := onBoard(t, e, 0, apaIsland)
	onBoard(t, e, 0, apaIsland)
	onBoard(t, e, 0, bounty)
	onBoard(t, e, 0, tax)
	onBoard(t, e, 0, apaDrone)
	pick := func(kind string, obj state.ObjID) {
		t.Helper()
		for _, o := range ppPriority(t, e).Options {
			if o.Kind == kind && o.Obj == obj {
				submitChoices(t, e, o.Index)
				return
			}
		}
		t.Fatalf("no %s of %d: %+v", kind, obj, e.Pending().Options)
	}
	pick("activate", land)
	pick("cast", id)
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KTarget {
		t.Fatalf("pending %+v, want the target ask", ask)
	}
	for _, o := range ask.Options {
		if o.Kind == "player" {
			return
		}
	}
	t.Errorf("withheld the player targets the Islands can pay for: %+v", ask.Options)
}
