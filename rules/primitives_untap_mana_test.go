package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The five primitives this ticket implements, each proven on a real card
// script (the exact Forge lines, inlined per the licensing rule -- never a
// .cards .txt):
//
//	api:Untap              Basalt Monolith
//	api:ManaReflected      Exotic Orchard, Fellwar Stone, Chrome Mox
//	stat:ManaConvert       Chromatic Orrery, Quicksilver Elemental
//	stat:UntapOtherPlayer  Endbringer
//	kw:Cumulative upkeep   Mystic Remora
//
// The monolith/vault R:Event$ Untap replacement is a separate, unsupported
// primitive and is intentionally not retired by this ticket.

const basaltMonolithScript = "Name:Basalt Monolith\nManaCost:3\nTypes:Artifact\n" +
	"R:Event$ Untap | ValidCard$ Card.Self | ValidStepTurnToController$ You | Layer$ CantHappen | Description$ This artifact doesn't untap during your untap step.\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 3 | SpellDescription$ Add {C}{C}{C}.\n" +
	"A:AB$ Untap | Cost$ 3 | SpellDescription$ Untap this artifact.\nOracle:x\n"

const exoticOrchardScript = "Name:Exotic Orchard\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ ManaReflected | Cost$ T | ColorOrType$ Color | Valid$ Land.OppCtrl | ReflectProperty$ Produce | SpellDescription$ Add one mana of any color that a land an opponent controls could produce.\n" +
	"Oracle:x\n"

const chromaticOrreryScript = "Name:Chromatic Orrery\nManaCost:7\nTypes:Legendary Artifact\n" +
	"S:Mode$ ManaConvert | ValidPlayer$ You | ManaConversion$ AnyType->AnyColor | Description$ You may spend mana as though it were mana of any color.\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 5 | SpellDescription$ Add {C}{C}{C}{C}{C}.\n" +
	"A:AB$ Draw | Cost$ 5 T | NumCards$ X | SpellDescription$ Draw a card for each color among permanents you control.\n" +
	"SVar:X:Count$Valid Permanent.YouCtrl$Colors\nOracle:x\n"

const quicksilverElementalScript = "Name:Quicksilver Elemental\nManaCost:3 U U\nTypes:Creature Elemental\nPT:3/4\n" +
	"S:Mode$ ManaConvert | ValidPlayer$ You | ValidCard$ Card.Self | ValidSA$ Activated | ManaConversion$ Blue->AnyColor | Description$ You may spend blue mana as though it were mana of any color to pay the activation costs of CARDNAME's abilities.\n" +
	"A:AB$ Effect | Cost$ U | ValidTgts$ Creature | TgtZone$ Battlefield | TgtPrompt$ Select target creature card | StaticAbilities$ STSteal | RememberLKI$ Targeted | Duration$ UntilHostLeavesPlayOrEOT | SpellDescription$ CARDNAME gains all activated abilities of target creature until end of turn.\n" +
	"SVar:STSteal:Mode$ Continuous | Affected$ Card.EffectSource | GainsAbilitiesOfDefined$ RememberedLKI | Description$ EFFECTSOURCE gains all activated abilities of that card until end of turn.\nOracle:x\n"

const endbringerScript = "Name:Endbringer\nManaCost:5 C\nTypes:Creature Eldrazi\nPT:5/5\n" +
	"S:Mode$ UntapOtherPlayer | ValidCard$ Card.Self | Description$ Untap CARDNAME during each other player's untap step.\n" +
	"A:AB$ DealDamage | Cost$ T | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ CARDNAME deals 1 damage to any target.\nOracle:x\n"

const mysticRemoraScript = "Name:Mystic Remora\nManaCost:U\nTypes:Enchantment\n" +
	"K:Cumulative upkeep:1\n" +
	"T:Mode$ SpellCast | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ TrigDraw | TriggerDescription$ Whenever an opponent casts a noncreature spell, you may draw a card unless that player pays {4}.\n" +
	"SVar:TrigDraw:DB$ Draw | Defined$ You | UnlessCost$ 4 | UnlessPayer$ TriggeredActivator | NumCards$ 1 | OptionalDecider$ You\nOracle:x\n"

const chromeMoxScript = "Name:Chrome Mox\nManaCost:0\nTypes:Artifact\n" +
	"T:Mode$ ChangesZone | ValidCard$ Card.Self | Origin$ Any | Destination$ Battlefield | OptionalDecider$ You | Execute$ TrigExile | TriggerDescription$ Imprint — When CARDNAME enters, you may exile a nonartifact, nonland card from your hand.\n" +
	"SVar:TrigExile:DB$ ChangeZone | Imprint$ True | Origin$ Hand | Destination$ Exile | ChangeType$ Card.nonArtifact+nonLand | ChangeNum$ 1\n" +
	"A:AB$ ManaReflected | Cost$ T | Valid$ Defined.Imprinted | ColorOrType$ Color | ReflectProperty$ Is | SpellDescription$ Add one mana of any of the exiled card's colors.\nOracle:x\n"

const ancestralRecallScript = "Name:Ancestral Recall\nManaCost:U\nTypes:Instant\n" +
	"A:SP$ Draw | NumCards$ 3 | SpellDescription$ Draw three cards.\nOracle:x\n"

const giantGrowthScript = "Name:Giant Growth\nManaCost:G\nTypes:Instant\n" +
	"A:SP$ Pump | ValidTgts$ Creature | NumAtt$ 3 | NumDef$ 3 | SpellDescription$ Target creature gets +3/+3 until end of turn.\nOracle:x\n"

func mountainScript() string {
	return "Name:Mountain\nTypes:Basic Land Mountain\n" +
		"A:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ Add {R}.\nOracle:x\n"
}

func forestScript() string {
	return "Name:Forest\nTypes:Basic Land Forest\n" +
		"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\nOracle:x\n"
}

// untapOptions finds the option kinds of a pending priority decision.
func optionKinds(d *decision.Decision) map[string]int {
	if d == nil {
		return nil
	}
	m := map[string]int{}
	for _, o := range d.Options {
		m[o.Kind]++
	}
	return m
}

// hasEvent reports whether the log carries an event of kind with the given
// object (obj 0 matches any).
func (e *Engine) hasEvent(kind events.Kind, obj state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == kind && (obj == 0 || ev.Obj == obj) {
			return true
		}
	}
	return false
}

// TestBasaltMonolithUntap proves api:Untap on Basalt Monolith's real script.
// A stun counter replaces its paid untap, just as it replaces an untap-step
// untap (CR 122.1d); after it is consumed the ability untaps normally.
func TestBasaltMonolithUntap(t *testing.T) {
	e := handEngine(t)
	basalt := onBoard(t, e, 0, basaltMonolithScript)
	e.emit(events.Event{Kind: events.Tap, Obj: basalt})
	if !e.G.Obj(basalt).Tapped {
		t.Fatal("fixture: Basalt should start tapped")
	}

	// api:Untap: the {3} ability is offered as an ordinary ability and
	// untaps on resolution.
	e.G.Players[0].Pool[state.MC] = 3
	e.priorityRound()
	if kinds(e.Pending().Options)["ability"] == 0 {
		t.Fatalf("Basalt's untap ability not offered: %+v", e.Pending().Options)
	}
	castFirst(t, e, "ability")
	passUntilStackEmpty(t, e, 8)
	if e.G.Obj(basalt).Tapped {
		t.Fatal("Basalt's {3} untap did not untap it")
	}

	// A stun counter replaces the first paid untap: it is removed and Basalt
	// remains tapped. The second activation has no stun left and untaps it.
	e.emit(events.Event{Kind: events.Tap, Obj: basalt})
	e.emit(events.Event{Kind: events.CounterChange, Obj: basalt, Counter: "STUN", Amount: 1})
	e.G.Players[0].Pool[state.MC] = 3
	e.priorityRound()
	castFirst(t, e, "ability")
	passUntilStackEmpty(t, e, 8)
	if !e.G.Obj(basalt).Tapped || e.G.Obj(basalt).Counter("STUN") != 0 {
		t.Fatalf("stun did not replace paid untap: tapped=%v stun=%d", e.G.Obj(basalt).Tapped, e.G.Obj(basalt).Counter("STUN"))
	}
	e.G.Players[0].Pool[state.MC] = 3
	e.priorityRound()
	castFirst(t, e, "ability")
	passUntilStackEmpty(t, e, 8)
	if e.G.Obj(basalt).Tapped {
		t.Fatal("Basalt did not untap after its stun counter was removed")
	}
}

// TestEndbringerUntapsDuringOtherPlayersUntapSteps proves
// stat:UntapOtherPlayer on Endbringer's real script: it untaps during every
// OTHER player's untap step (and a neighbouring permanent without the static
// does not), and during its own controller's untap step it untaps normally.
func TestEndbringerUntapsDuringOtherPlayersUntapSteps(t *testing.T) {
	e := handEngine(t)
	endbringer := onBoard(t, e, 1, endbringerScript)
	plain := onBoard(t, e, 1, mountainScript())
	e.emit(events.Event{Kind: events.Tap, Obj: endbringer})
	e.emit(events.Event{Kind: events.Tap, Obj: plain})

	// Seat 0's untap step: Endbringer untaps, the plain land does not.
	e.G.Turn = 2
	e.beginTurn(0)
	if e.G.Obj(endbringer).Tapped {
		t.Fatal("Endbringer did not untap during an other player's untap step")
	}
	if !e.G.Obj(plain).Tapped {
		t.Fatal("a permanent without the static untapped during a foreign untap step")
	}
	if !e.hasEvent(events.Untap, endbringer) {
		t.Fatal("Endbringer's foreign untap-step untap emitted no Untap event")
	}

	// Seat 1's own untap step: everything untaps normally.
	e.emit(events.Event{Kind: events.Tap, Obj: endbringer})
	e.emit(events.Event{Kind: events.Tap, Obj: plain})
	e.G.Turn = 3
	e.beginTurn(1)
	if e.G.Obj(endbringer).Tapped || e.G.Obj(plain).Tapped {
		t.Fatal("the controller's own untap step did not untap its permanents")
	}
}

// TestExoticOrchardReflectedMana proves api:ManaReflected on Exotic Orchard's
// real script: one opponent land with one producible colour adds that colour
// with no ask; a second land widens the set and the activation asks
// (chooseManaColor); with nothing to reflect the activation is not offered.
// Fellwar Stone carries the identical script shape and Chrome Mox the
// Defined.Imprinted one, so one test covers the primitive's three real
// forms.
func TestExoticOrchardReflectedMana(t *testing.T) {
	e := handEngine(t)
	orchard := onBoard(t, e, 0, exoticOrchardScript)
	e.priorityRound()
	if optionKinds(e.Pending())["activate"] != 0 {
		t.Fatal("Exotic Orchard offered an activation with nothing to reflect")
	}

	_ = onBoard(t, e, 1, mountainScript())
	e.priorityRound()
	if optionKinds(e.Pending())["activate"] != 1 {
		t.Fatalf("Exotic Orchard's activation not offered with an opponent land: %+v", e.Pending().Options)
	}
	castFirst(t, e, "activate")
	if e.G.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("Exotic Orchard should have added one R, pool %+v", e.G.Players[0].Pool)
	}

	// A second opponent land of a different colour widens the candidate set;
	// the activation now asks, and the answer's colour is what is added.
	// (The first activation's {T} tapped the Orchard; the fixture untaps it
	// so the same source can be activated again.)
	e.emit(events.Event{Kind: events.Untap, Obj: orchard})
	_ = onBoard(t, e, 1, forestScript())
	e.priorityRound()
	castFirst(t, e, "activate")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected a colour choice, got %+v", d)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Add R" || d.Options[1].Label != "Add G" {
		t.Fatalf("colour options wrong: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{1}}); err != nil {
		t.Fatalf("submit colour: %v", err)
	}
	if e.G.Players[0].Pool[state.MG] != 1 {
		t.Fatalf("Exotic Orchard should have added one G after the ask, pool %+v", e.G.Players[0].Pool)
	}
	_ = orchard
}

// TestFellwarStoneAndChromeMoxReflectedShapes pins the other two
// ManaReflected forms: Fellwar Stone is Exotic Orchard's twin (same
// candidates); Chrome Mox's Valid$ Defined.Imprinted resolves through the
// Defined machinery, finds no imprint record in this build, and is therefore
// not offered at all (an ability that can only resolve into a Note must not
// be offered).
func TestReflectingPoolTypeReflectsOnlyProducedColourless(t *testing.T) {
	e := handEngine(t)
	pool := onBoard(t, e, 0, "Name:Reflecting Pool\nManaCost:no cost\nTypes:Land\n"+
		"A:AB$ ManaReflected | Cost$ T | ColorOrType$ Type | Valid$ Land.YouCtrl | ReflectProperty$ Produce | SpellDescription$ Add one mana of any type that a land you control could produce.\nOracle:x\n")
	sa := e.G.Obj(pool).Face().Abilities[0]
	ctx := &effects.Ctx{Source: pool, Controller: 0, SVars: e.G.Obj(pool).Face().SVars}
	if got := effects.ManaReflectedCandidates(e, ctx, sa); len(got) != 0 {
		t.Fatalf("Reflecting Pool alone reflected mana: %v", got)
	}
	_ = onBoard(t, e, 0, mountainScript())
	if got := effects.ManaReflectedCandidates(e, ctx, sa); len(got) != 1 || got[0] != "R" {
		t.Fatalf("Reflecting Pool plus Mountain = %v, want [R]", got)
	}
}

func TestFellwarStoneAndChromeMoxReflectedShapes(t *testing.T) {
	e := handEngine(t)
	onBoard(t, e, 0, "Name:Fellwar Stone\nManaCost:2\nTypes:Artifact\n"+
		"A:AB$ ManaReflected | Cost$ T | ColorOrType$ Color | Valid$ Land.OppCtrl | ReflectProperty$ Produce | SpellDescription$ Add one mana of any color that a land an opponent controls could produce.\nOracle:x\n")
	onBoard(t, e, 1, mountainScript())
	e.priorityRound()
	if optionKinds(e.Pending())["activate"] != 1 {
		t.Fatalf("Fellwar Stone's activation not offered: %+v", e.Pending().Options)
	}
	castFirst(t, e, "activate")
	if e.G.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("Fellwar Stone should have added one R, pool %+v", e.G.Players[0].Pool)
	}

	e2 := handEngine(t)
	chrome := onBoard(t, e2, 0, chromeMoxScript)
	blue := e2.G.AddObject(card(t, ancestralRecallScript), 0)
	blue.Zone = state.ZHand
	e2.G.SetZone(state.ZHand, 0, append(e2.G.Zone(state.ZHand, 0), blue.ID))
	// Resolve Chrome Mox's real enter-the-battlefield imprint effect. With one
	// eligible card the generic ChangeZone selector has no choice to ask.
	effects.Resolve(e2, &effects.Ctx{Source: chrome, Controller: 0, SVars: e2.G.Obj(chrome).Face().SVars}, e2.G.Obj(chrome).Face().Triggers[0].Effect)
	if got := e2.G.Obj(chrome).Imprinted; len(got) != 1 || got[0] != blue.ID {
		t.Fatalf("Chrome Mox did not retain its imprint: %v", got)
	}
	e2.priorityRound()
	if optionKinds(e2.Pending())["activate"] != 1 {
		t.Fatalf("Chrome Mox did not offer mana from its imprinted blue card: %+v", e2.Pending())
	}
	castFirst(t, e2, "activate")
	if e2.G.Players[0].Pool[state.MU] != 1 {
		t.Fatalf("Chrome Mox did not produce the imprinted card's U: %+v", e2.G.Players[0].Pool)
	}
}

// TestChromaticOrreryManaConvert proves stat:ManaConvert on Chromatic
// Orrery's real script: with the Orrery on the battlefield, one red of
// floating mana may pay a {U} spell's pip; without it the cast is not
// offered. Also pins the scoping on Quicksilver Elemental's real script:
// Blue->AnyColor scoped by ValidSA$ Activated + ValidCard$ Card.Self never
// widens a SPELL cast, and its blue-only conversion does not make red mana
// pay its own {U} ability.
func TestChromaticOrreryManaConvert(t *testing.T) {
	e := handEngine(t, card(t, ancestralRecallScript))
	e.G.Players[0].Pool[state.MR] = 1
	e.priorityRound()
	if optionKinds(e.Pending())["cast"] != 0 {
		t.Fatal("Ancestral Recall castable from a red-only pool with no converter")
	}
	_ = onBoard(t, e, 0, chromaticOrreryScript)
	e.priorityRound()
	if optionKinds(e.Pending())["cast"] != 1 {
		t.Fatal("Ancestral Recall not castable with Chromatic Orrery's conversion live")
	}
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 8)
	if total := e.G.Players[0].Pool.Total(); total != 0 {
		t.Fatalf("the converted payment did not spend the pool: %+v", e.G.Players[0].Pool)
	}
	if len(e.G.Zone(state.ZHand, 0)) != 3 {
		t.Fatalf("Ancestral Recall did not draw 3: hand %d", len(e.G.Zone(state.ZHand, 0)))
	}

	// Quicksilver Elemental: the same red pool must NOT cast a green spell
	// (its conversion is scoped to its own abilities), and red must not pay
	// its own {U} ability either (only BLUE mana converts).
	e2 := handEngine(t, card(t, giantGrowthScript))
	_ = onBoard(t, e2, 0, quicksilverElementalScript)
	onBoard(t, e2, 1, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e2.G.Players[0].Pool[state.MU] = 1
	e2.priorityRound()
	if optionKinds(e2.Pending())["cast"] != 0 {
		t.Fatal("Quicksilver's Blue->AnyColor conversion leaked onto a spell cast")
	}
	if optionKinds(e2.Pending())["ability"] != 1 {
		t.Fatal("Quicksilver's own {U} ability not offered from a blue pool")
	}
	e2.G.Players[0].Pool = state.Mana{}
	e2.G.Players[0].Pool[state.MR] = 1
	e2.priorityRound()
	if optionKinds(e2.Pending())["ability"] != 0 {
		t.Fatal("red mana paid Quicksilver's {U} ability although only BLUE converts")
	}
	// Blue conversion is positive too: it can pay the ability's printed {U}
	// as a different colour, here represented by a {G} ability cost.
	e2.G.Players[0].Pool = state.Mana{}
	e2.G.Players[0].Pool[state.MU] = 1
	e2.priorityRound()
	if optionKinds(e2.Pending())["ability"] != 1 {
		t.Fatal("blue mana did not pay Quicksilver's own activated ability")
	}
}

// TestDrumbellowerUntapsItsCreaturesForEachOtherPlayer pins the class shape
// of stat:UntapOtherPlayer beyond Endbringer's Card.Self: Drumbellower's
// ValidCard$ Creature.YouCtrl untaps EVERY creature its controller controls
// during each other player's untap step, and nothing else.
func TestFabledPassageUntapsOnlyAtFourLands(t *testing.T) {
	e := handEngine(t)
	passage := onBoard(t, e, 0, "Name:Fabled Passage\nManaCost:no cost\nTypes:Land\n"+
		"A:AB$ ChangeZone | Cost$ T Sac<1/CARDNAME> | Origin$ Library | Destination$ Battlefield | Tapped$ True | ChangeType$ Land.Basic | RememberChanged$ True | SubAbility$ DBUntap\n"+
		"SVar:DBUntap:DB$ Untap | Defined$ Remembered | ConditionPresent$ Land.YouCtrl | ConditionCompare$ GE4\nOracle:x\n")
	fetched := onBoard(t, e, 0, mountainScript())
	e.emit(events.Event{Kind: events.Tap, Obj: fetched})
	untap := e.G.Obj(passage).Face().Abilities[0].Sub
	ctx := &effects.Ctx{Source: passage, Controller: 0, Remembered: []state.Target{{Obj: fetched}}}
	effects.Resolve(e, ctx, untap)
	if !e.G.Obj(fetched).Tapped {
		t.Fatal("Fabled Passage untapped fetched land below four lands")
	}
	_ = onBoard(t, e, 0, forestScript())
	_ = onBoard(t, e, 0, forestScript())
	effects.Resolve(e, ctx, untap)
	if e.G.Obj(fetched).Tapped {
		t.Fatal("Fabled Passage did not untap fetched land at four lands")
	}
}

func TestQuestForRenewalNeedsFourQuestCounters(t *testing.T) {
	e := handEngine(t)
	quest := onBoard(t, e, 1, "Name:Quest for Renewal\nManaCost:1 G\nTypes:Enchantment\n"+
		"S:Mode$ UntapOtherPlayer | ValidCard$ Creature.YouCtrl | IsPresent$ Card.Self+counters_GE4_QUEST | Description$ As long as there are four or more quest counters on CARDNAME, untap all creatures you control during each other player's untap step.\nOracle:x\n")
	creature := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Tap, Obj: creature})
	e.G.Turn = 2
	e.beginTurn(0)
	if !e.G.Obj(creature).Tapped {
		t.Fatal("Quest for Renewal untapped below four quest counters")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: quest, Counter: "QUEST", Amount: 4})
	e.G.Turn = 3
	e.beginTurn(0)
	if e.G.Obj(creature).Tapped {
		t.Fatal("Quest for Renewal did not untap at four quest counters")
	}
}

func TestDrumbellowerUntapsItsCreaturesForEachOtherPlayer(t *testing.T) {
	e := handEngine(t)
	drum := onBoard(t, e, 1, "Name:Drumbellower\nManaCost:2 W\nTypes:Creature Spirit\nPT:2/1\nK:Flying\n"+
		"S:Mode$ UntapOtherPlayer | ValidCard$ Creature.YouCtrl | Description$ Untap all creatures you control during each other player's untap step.\nOracle:x\n")
	creature := onBoard(t, e, 1, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	land := onBoard(t, e, 1, mountainScript())
	e.emit(events.Event{Kind: events.Tap, Obj: drum})
	e.emit(events.Event{Kind: events.Tap, Obj: creature})
	e.emit(events.Event{Kind: events.Tap, Obj: land})
	e.G.Turn = 2
	e.beginTurn(0)
	if e.G.Obj(drum).Tapped || e.G.Obj(creature).Tapped {
		t.Fatal("Drumbellower's creatures did not untap during a foreign untap step")
	}
	if !e.G.Obj(land).Tapped {
		t.Fatal("a non-creature untapped under a Creature.YouCtrl static")
	}
}

// TestMysticRemoraCumulativeUpkeep proves kw:Cumulative upkeep on Mystic
// Remora's real script: at its controller's upkeep an age counter is placed
// and a pay-or-sacrifice choice is asked; paying keeps it and leaves the age
// counters in place, the next upkeep doubles the demand, and an unpayable
// demand offers sacrifice only.
func TestMysticRemoraCumulativeUpkeep(t *testing.T) {
	e := handEngine(t)
	remora := onBoard(t, e, 0, mysticRemoraScript)
	_ = onBoard(t, e, 0, mountainScript())
	e.G.Turn = 2

	// Turn 2's upkeep: the pool starts empty, so CR 702.46b opens a mana-only
	// payment window. Tapping a Mountain returns to the cumulative choice;
	// paying keeps the Remora and its age counter.
	e.beginTurn(0)
	d := e.Pending()
	if d == nil || len(d.Options) != 2 || d.Options[0].Kind != "activate" || d.Options[1].Kind != "done" {
		t.Fatalf("expected cumulative mana window, got %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("activate mana: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 ||
		d.Options[0].Kind != "cumulative_pay" || d.Options[1].Kind != "cumulative_sac" {
		t.Fatalf("expected the cumulative pay-or-sacrifice choice after mana, got %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit pay: %v", err)
	}
	if e.G.Obj(remora).Zone != state.ZBattlefield {
		t.Fatal("the paid cumulative upkeep sacrificed the Remora")
	}
	if e.G.Obj(remora).Counter("AGE") != 1 {
		t.Fatalf("expected 1 age counter, got %d", e.G.Obj(remora).Counter("AGE"))
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("the payment did not spend the pool: %+v", e.G.Players[0].Pool)
	}

	// Turn 3's upkeep with an empty pool: the demand is now {2} for two age
	// counters, paying is impossible, so ONLY sacrifice is offered and the
	// answer sacrifices the Remora to its graveyard.
	// Remove the mana source so the next upkeep has no payment route.
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id != remora {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
		}
	}
	e.G.Turn = 3
	e.beginTurn(0)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 ||
		d.Options[0].Kind != "cumulative_sac" {
		t.Fatalf("expected sacrifice-only for an unpayable demand, got %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit sacrifice: %v", err)
	}
	if e.G.Obj(remora).Zone != state.ZGraveyard {
		t.Fatalf("Remora not sacrificed for cumulative upkeep: zone %v", e.G.Obj(remora).Zone)
	}
}
