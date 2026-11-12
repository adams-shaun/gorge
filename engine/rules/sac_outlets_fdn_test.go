package rules

// Focused, ordinary-suite coverage for the three FDN sacrifice cards
// the cold-start card-value spike rated far below 17Lands (Hungry Ghoul,
// Vampire Gourmand, Fake Your Own Death). The data-driven oracle audit
// (engine/rules/testdata/oracle/...) already drives each of them end to end; these
// leaves pin the same rules text natively so a regression names the card in
// the ordinary suite.
//
// Every leaf asserts its own precondition first (the carrier and its fodder
// are on the battlefield, the ability is offered exactly when it should be),
// so a vacuous board fails loudly rather than passing silently.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cv7CountNamed counts seat p's battlefield permanents whose face is named
// name (used for the Treasure token assertion).
func cv7CountNamed(e *Engine, p state.PlayerID, name string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && strings.Contains(o.Face().Name, name) {
			n++
		}
	}
	return n
}

// cv7PickLabel submits the pending decision's first option whose Label
// contains substr -- the "Sacrifice another creature" yes-branch and the
// victim pick both arrive as KChoose asks with the card name as the label.
func cv7PickLabel(t *testing.T, e *Engine, substr string) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatalf("no decision pending while looking for option %q", substr)
	}
	for _, o := range d.Options {
		if strings.Contains(o.Label, substr) {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no option labelled %q in %+v", substr, d.Options)
}

// TestFDNHungryGhoulSacrificesAnotherCreatureForACounter pins Hungry Ghoul's
// `{1}, Sacrifice another creature: Put a +1/+1 counter on this creature`:
// the {1} gates the offer, the sacrifice cost takes exactly the OTHER
// creature, and the payoff is one +1/+1 counter.
func TestFDNHungryGhoulSacrificesAnotherCreatureForACounter(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ghoul := registryCardOrFatal(t, reg, "Hungry Ghoul")
	bear := card(t, staticBearFixture)

	e, cfg := restrictionGame(t, 9701,
		[][]*cards.Card{{}, {}},
		[][]*cards.Card{{ghoul, bear}, {}})
	ghoulID := battlefieldObj(t, e, 0, ghoul)
	bearID := battlefieldObj(t, e, 0, bear)
	if ghoulID == bearID {
		t.Fatal("precondition: the Ghoul and its fodder must be distinct objects")
	}

	// The {1} is part of the cost: with an empty pool the ability is not
	// offered, so the offer below cannot be a board artefact.
	if _, ok := findAbilityOption(e, ghoulID, 0); ok {
		t.Fatalf("precondition: the {1} cost must withhold the ability with an empty pool: %+v", e.Pending())
	}

	addMana(t, e, 0, "C")
	opt, ok := findAbilityOption(e, ghoulID, 0)
	if !ok {
		t.Fatalf("Hungry Ghoul's ability is not offered with {1} available: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)

	// The sacrifice cost is paid through a choose ask whose only legal
	// candidate is the OTHER creature -- the Ghoul must never be offered as
	// its own sacrifice.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the sacrifice-target choose, got %+v", d)
	}
	victim := -1
	for _, o := range d.Options {
		if o.Obj == ghoulID {
			t.Fatalf("the Ghoul must not be offered as its own sacrifice: %+v", d.Options)
		}
		if o.Obj == bearID {
			victim = o.Index
		}
	}
	if victim < 0 {
		t.Fatalf("the bear is not offered as the sacrifice candidate: %+v", d.Options)
	}
	submitChoices(t, e, victim)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(bearID).Zone; got != state.ZGraveyard {
		t.Fatalf("the sacrificed bear is in %s, want graveyard", got)
	}
	if got := e.G.Obj(ghoulID).Counter("P1P1"); got != 1 {
		t.Fatalf("the Ghoul has %d +1/+1 counters, want 1", got)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("the {1} was not spent: pool %v", e.G.Players[0].Pool)
	}
	replayCheck(t, e, cfg)
}

// TestFDNHungryGhoulCannotSacrificeItselfOrANonCreature pins the "another
// creature" restriction from the other side: with the Ghoul as its
// controller's only creature (even though the opponent has one), and with
// only a non-creature permanent as a would-be fodder, the ability is not
// offered.
func TestFDNHungryGhoulCannotSacrificeItselfOrANonCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ghoul := registryCardOrFatal(t, reg, "Hungry Ghoul")
	bear := card(t, staticBearFixture)
	rock := card(t, "Name:CV7 Rock\nManaCost:0\nTypes:Artifact\nOracle:x\n")

	// Alone: the opponent's bear is not "another creature" the Ghoul controls.
	alone, aloneCfg := restrictionGame(t, 9702,
		[][]*cards.Card{{}, {}},
		[][]*cards.Card{{ghoul}, {bear}})
	aloneGhoul := battlefieldObj(t, alone, 0, ghoul)
	addMana(t, alone, 0, "CC")
	if _, ok := findAbilityOption(alone, aloneGhoul, 0); ok {
		t.Fatalf("Hungry Ghoul must not be activatable with no OTHER creature of its controller's: %+v", alone.Pending())
	}
	replayCheck(t, alone, aloneCfg)

	// A non-creature permanent is not a legal `Sac<1/Creature.Other>` fodder.
	e, cfg := restrictionGame(t, 9703,
		[][]*cards.Card{{}, {}},
		[][]*cards.Card{{ghoul, rock}, {}})
	ghoulID := battlefieldObj(t, e, 0, ghoul)
	if battlefieldObj(t, e, 0, rock) == 0 {
		t.Fatal("precondition: the artifact must be on the battlefield")
	}
	addMana(t, e, 0, "CC")
	if _, ok := findAbilityOption(e, ghoulID, 0); ok {
		t.Fatalf("a non-creature permanent must not satisfy Sac<1/Creature.Other>: %+v", e.Pending())
	}
	replayCheck(t, e, cfg)
}

// TestFDNVampireGourmandAttackDrawsAndGoesUnblockable pins the attack
// trigger: on its own attack the controller may sacrifice another creature;
// doing so draws a card and the Gourmand can't be blocked this turn.
func TestFDNVampireGourmandAttackDrawsAndGoesUnblockable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gourmand := registryCardOrFatal(t, reg, "Vampire Gourmand")
	bear := card(t, staticBearFixture)
	courser := card(t, "Name:CV7 Courser\nManaCost:3 G\nTypes:Creature Beast\nPT:3/3\nOracle:x\n")

	e, cfg := restrictionGame(t, 9704,
		[][]*cards.Card{{}, {}},
		[][]*cards.Card{{gourmand, bear}, {courser}})
	gourID := battlefieldObj(t, e, 0, gourmand)
	bearID := battlefieldObj(t, e, 0, bear)
	blkID := battlefieldObj(t, e, 1, courser)
	e.G.Obj(gourID).SummonSick = false
	hand := len(e.G.Zone(state.ZHand, 0))

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackersOnly(t, e, gourID)
	passPriorityUntilAsk(t, e, "Sacrifice")
	cv7PickLabel(t, e, "Sacrifice another creature")
	// The bear is the only OTHER creature, so its selection is a singleton
	// and the engine takes it without a second ask.
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(bearID).Zone; got != state.ZGraveyard {
		t.Fatalf("the sacrificed bear is in %s, want graveyard", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
		t.Fatalf("hand after the sacrifice = %d, want %d (the draw)", got, hand+1)
	}
	if e.canBlock(blkID, gourID) {
		t.Fatal("Vampire Gourmand must be unblockable after the sacrifice")
	}
	replayCheck(t, e, cfg)
}

// TestFDNFakeYourOwnDeathReturnsTappedWithATreasure pins the payoff: the
// spell gives +2/+0 until end of turn and grants "when this creature dies,
// return it tapped and create a Treasure"; the returned creature is a new,
// undamaged object.
func TestFDNFakeYourOwnDeathReturnsTappedWithATreasure(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bear := card(t, staticBearFixture)
	fyod := registryCardOrFatal(t, reg, "Fake Your Own Death")
	bolt := registryCardOrFatal(t, reg, "Lightning Bolt")

	e, cfg := restrictionGame(t, 9705,
		[][]*cards.Card{{fyod, bolt}, {}},
		[][]*cards.Card{{bear}, {}})
	bearID := battlefieldObj(t, e, 0, bear)
	fyodID := findAndMoveToHand(t, e, 0, "Fake Your Own Death")
	boltID := findAndMoveToHand(t, e, 0, "Lightning Bolt")

	addMana(t, e, 0, "CB")
	castFromPriority(t, e, fyodID)
	answerKTarget(t, e, bearID)
	passUntilStackEmpty(t, e, 20)
	if got := e.Power(bearID); got != 4 {
		t.Fatalf("bear power after Fake Your Own Death = %d, want 4 (+2/+0)", got)
	}

	addMana(t, e, 0, "R")
	castFromPriority(t, e, boltID)
	answerKTarget(t, e, bearID)
	passUntilStackEmpty(t, e, 20)

	returned := battlefieldObj(t, e, 0, bear)
	o := e.G.Obj(returned)
	if !o.Tapped {
		t.Fatal("the returned bear must be tapped")
	}
	if o.Damage != 0 {
		t.Fatalf("the returned bear has %d damage, want 0 (a new object)", o.Damage)
	}
	if n := cv7CountNamed(e, 0, "Treasure"); n != 1 {
		t.Fatalf("Treasures on seat 0 = %d, want 1", n)
	}
	if n := cv7CountNamed(e, 1, "Treasure"); n != 0 {
		t.Fatalf("Treasures on seat 1 = %d, want 0", n)
	}
	replayCheck(t, e, cfg)
}
