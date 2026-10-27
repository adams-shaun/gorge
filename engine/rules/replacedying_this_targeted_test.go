package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Carbonize exercises the qualified ThisTargetedCard.Creature selector at
// DealDamage's shared replacement-registration point.
func TestDealDamageReplaceDyingThisTargetedCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	carbonize := mustCorpusCard(t, reg, "Carbonize")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := censusEngine(t, 8401, []*cards.Card{carbonize}, []*cards.Card{bear})
	cardToHand(t, e, carbonize)
	bearID := moveOwnerCard(t, e, 1, bear, state.ZBattlefield)
	if o := e.G.Obj(bearID); o.Zone != state.ZBattlefield || e.Toughness(bearID) != 2 {
		t.Fatalf("precondition: target bear is zone %s with toughness %d, want battlefield 2/2", o.Zone, e.Toughness(bearID))
	}
	addMana(t, e, 0, "RRR")
	castFirst(t, e, "cast")
	targetObject(t, e, bearID)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bearID); o.Zone != state.ZExile {
		t.Fatalf("targeted lethal-damage bear should be exiled, got %s", o.Zone)
	}
	assertReplaceDyingRemembers(t, e, bearID)
	replayCheck(t, e, cfg)
}

// Unnatural Aggression's Fight damages both participants, but its bare
// ThisTargetedCard replacement belongs only to the opponent's chosen target.
func TestFightReplaceDyingThisTargetedCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	aggression := mustCorpusCard(t, reg, "Unnatural Aggression")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	giant := mustCorpusCard(t, reg, "Hill Giant")
	e, cfg := censusEngine(t, 8402, []*cards.Card{aggression, giant}, []*cards.Card{bear})
	cardToHand(t, e, aggression)
	fighter := moveOwnerCard(t, e, 0, giant, state.ZBattlefield)
	target := moveOwnerCard(t, e, 1, bear, state.ZBattlefield)
	if o := e.G.Obj(fighter); o.Zone != state.ZBattlefield || e.Power(fighter) <= e.Toughness(target) {
		t.Fatalf("precondition: fighter zone=%s power=%d, target toughness=%d; expected battlefield fighter whose power is lethal", o.Zone, e.Power(fighter), e.Toughness(target))
	}
	if o := e.G.Obj(target); o.Zone != state.ZBattlefield || e.Power(target) >= e.Toughness(fighter) {
		t.Fatalf("precondition: target zone=%s power=%d, fighter toughness=%d; expected battlefield target that does not kill fighter", o.Zone, e.Power(target), e.Toughness(fighter))
	}
	addMana(t, e, 0, "2GG")
	castFirst(t, e, "cast")
	targetObject(t, e, fighter)
	targetObject(t, e, target)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(target); o.Zone != state.ZExile {
		t.Fatalf("targeted opponent creature should be exiled, got %s", o.Zone)
	}
	if o := e.G.Obj(fighter); o.Zone != state.ZBattlefield {
		t.Fatalf("non-target fighter should remain on battlefield, got %s", o.Zone)
	}
	assertReplaceDyingRemembers(t, e, target)
	replayCheck(t, e, cfg)
}

// TestFightReplaceDyingTargetedExilesOnlyTheChosenTarget pins Faunsbane
// Troll's activated fight: `DB$ Fight | Defined$ Self | ValidTgts$
// Creature.YouDontCtrl | ReplaceDyingDefined$ Targeted`. Both fighters die,
// but `Targeted` names only the chosen creature, so only it is exiled; the
// Troll goes to its owner's graveyard.
func TestFightReplaceDyingTargetedExilesOnlyTheChosenTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	trollC := mustCorpusCard(t, reg, "Faunsbane Troll")
	wurmC := mustCorpusCard(t, reg, "Craw Wurm")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{trollC}, []*cards.Card{wurmC})
	troll := moveCorpusCard(t, e, "Faunsbane Troll", 0, state.ZBattlefield)
	wurm := moveCorpusCard(t, e, "Craw Wurm", 1, state.ZBattlefield)
	if o := e.G.Obj(troll); o == nil || o.Zone != state.ZBattlefield || e.Power(troll) < 4 {
		t.Fatalf("precondition: Faunsbane Troll is not a 4-power battlefield creature: %+v", o)
	}
	if o := e.G.Obj(wurm); o == nil || o.Zone != state.ZBattlefield || e.Power(wurm) < 4 || e.Toughness(wurm) > 5 {
		t.Fatalf("precondition: Craw Wurm must be a battlefield creature that kills and is killed by the Troll: %+v", o)
	}
	// Resolve the Troll's ETB so its Monster Role Aura exists: that Aura is
	// the permanent the activation's Sac<1/Aura.Attached> cost sacrifices.
	passUntilStackEmpty(t, e, 20)
	addMana(t, e, 0, "C")
	submitChoices(t, e, abilityOption(t, e, troll, 0).Index)
	fightDrain(t, e, false)
	if got := e.G.Obj(wurm).Zone; got != state.ZExile {
		t.Fatalf("the chosen target is in %s, want exile (ReplaceDyingDefined$ Targeted)", got)
	}
	if got := e.G.Obj(troll).Zone; got != state.ZGraveyard {
		t.Fatalf("Faunsbane Troll itself is in %s, want the graveyard: the Targeted selector names only the chosen target, not the Defined$ Self fighter", got)
	}
	replayCheck(t, e, cfg)
}

func assertReplaceDyingRemembers(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	n := 0
	for _, ce := range e.continuous {
		if ce.ReplacementEvent != "Moved" {
			continue
		}
		n++
		if len(ce.Remembered) != 1 || ce.Remembered[0] != want {
			t.Fatalf("replacement remembered %v, want only targeted object %d", ce.Remembered, want)
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one Moved replacement, got %d", n)
	}
}
