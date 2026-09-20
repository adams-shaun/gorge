package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Mutate (CR 702.140) pinned end to end on the filing card, Everquill
// Phoenix (K:Mutate:3 R, T:Mode$ Mutates ... create a Feather token): the
// mutate cast is offered beside the plain creature cast, pays the mutate cost
// instead of the mana cost, asks the over/under placement and a non-Human
// creature you own as its target, merges the cards into one permanent (the
// top card carries its characteristics, the cards beneath it are recorded),
// and fires the "whenever this creature mutates" trigger exactly once per
// mutation with Count$TimesMutated tracking the count.

const mutateBearSrc = "Name:Mutate Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// mutatedCastOption returns the "cast" option with Mode "mutated" for the id.
func mutatedCastOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode == "mutated" {
			return o
		}
	}
	t.Fatalf("no mutated cast option for card %d in %+v", id, castOptions(t, e))
	return decision.Option{}
}

// mutateCastOnto submits the mutated cast, answers CR 702.140b's placement
// ask (onTop selects the first "On top" option; otherwise "Under"), answers
// the non-Human-creature target ask with bearer, and drains the stack.
func mutateCastOnto(t *testing.T, e *Engine, opt decision.Option, bearer state.ObjID, onTop bool) {
	t.Helper()
	submitChoices(t, e, opt.Index)
	// CR 702.140b: the over/under placement is announced at cast time.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("mutate placement ask: %+v", d)
	}
	place := -1
	for _, o := range d.Options {
		if o.Kind != "mutate_place" {
			continue
		}
		if onTop && o.Label == "On top" {
			place = o.Index
		}
		if !onTop && o.Label == "Under" {
			place = o.Index
		}
	}
	if place < 0 {
		t.Fatalf("no %s placement option: %+v", map[bool]string{true: "on-top", false: "under"}[onTop], d.Options)
	}
	submitChoices(t, e, place)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("mutate target ask: %+v", d)
	}
	idx := indexOfObjOption(d, bearer)
	if idx < 0 {
		t.Fatalf("mutate target ask does not offer %d: %+v", bearer, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 40)
}

// battlefieldNamedCount is a thin alias for the shared foretell_test helper
// (which counts battlefield permanents named name under seat 0).
func battlefieldNamedCount(t *testing.T, e *Engine, name string) int {
	return battlefieldNamed(t, e, name)
}

// TestEverquillPhoenixMutatesOntoTopAndCreatesFeather is the filing card: the
// priority decision offers both the plain cast ({2}{R}{R}) and the mutate
// cast ({3}{R}), the mutate cast asks placement and a non-Human creature you
// own, and after resolution the surviving permanent is Everquill Phoenix on
// top of the bear with TimesMutated 1 -- and its "whenever this creature
// mutates" trigger has made the Feather token.
func TestEverquillPhoenixMutatesOntoTopAndCreatesFeather(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, cfg := tokenReplGame(t, 301, phoenix)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRRR") // the mutate cost {3}{R}

	if o := plainCastOption(t, e, phoenixID); o.Label != "Cast Everquill Phoenix" {
		t.Fatalf("plain cast label %q", o.Label)
	}
	if o := mutatedCastOption(t, e, phoenixID); o.Label != "Cast Everquill Phoenix (mutated)" {
		t.Fatalf("mutated cast label %q", o.Label)
	}

	mutateCastOnto(t, e, mutatedCastOption(t, e, phoenixID), bear, true)

	// The bear's object survived as the pile; its top card is now the Phoenix.
	pile := e.G.Obj(bear)
	if pile == nil || pile.Zone != state.ZBattlefield {
		t.Fatalf("mutated pile zone: %+v", pile)
	}
	if pile.Face() == nil || pile.Face().Name != "Everquill Phoenix" {
		t.Fatalf("mutated pile top card = %v, want Everquill Phoenix", pile.Face())
	}
	if pile.TimesMutated != 1 {
		t.Fatalf("TimesMutated = %d, want 1", pile.TimesMutated)
	}
	if len(pile.MergedCards) != 1 || pile.MergedCards[0].Card == nil ||
		pile.MergedCards[0].Card.Faces[0].Name != "Mutate Bear" {
		t.Fatalf("merged under-cards = %+v, want the bear beneath the Phoenix", pile.MergedCards)
	}
	if battlefieldNamedCount(t, e, "Feather") == 0 {
		t.Fatal("the Mutates trigger did not create the Feather token")
	} // The phoenix's own card object must NOT still be an independent
	// permanent: it merged into the pile.
	if o := e.G.Obj(phoenixID); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("the mutating card is still a separate battlefield permanent")
	}
	replayCheck(t, e, cfg)
}

// TestEverquillPhoenixMutatesUnderKeepsTargetOnTop: CR 702.140b's other
// choice -- the mutating card goes UNDER the target, so the surviving pile's
// top card is still the bear (its 2/2 characteristics) and the Phoenix is the
// recorded under-card.
func TestEverquillPhoenixMutatesUnderKeepsTargetOnTop(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, cfg := tokenReplGame(t, 302, phoenix)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRRR")

	mutateCastOnto(t, e, mutatedCastOption(t, e, phoenixID), bear, false)

	pile := e.G.Obj(bear)
	if pile.Face() == nil || pile.Face().Name != "Mutate Bear" {
		t.Fatalf("under-placement top card = %v, want the bear", pile.Face())
	}
	if p, tt := e.Power(bear), e.Toughness(bear); p != 2 || tt != 2 {
		t.Fatalf("under-placement P/T = %d/%d, want the bear's 2/2", p, tt)
	}
	if pile.TimesMutated != 1 {
		t.Fatalf("TimesMutated = %d, want 1", pile.TimesMutated)
	}
	if len(pile.MergedCards) != 1 || pile.MergedCards[0].Card == nil ||
		pile.MergedCards[0].Card.Faces[0].Name != "Everquill Phoenix" {
		t.Fatalf("merged under-cards = %+v, want the Phoenix beneath the bear", pile.MergedCards)
	}
	// CR 702.140d: the permanent has all abilities of the cards beneath it, so
	// the under-card Phoenix's "whenever this creature mutates" trigger fires
	// even though the bear is the top card.
	if battlefieldNamedCount(t, e, "Feather") == 0 {
		t.Fatal("the under-card Mutates trigger did not create the Feather token")
	}
	replayCheck(t, e, cfg)
}

// TestMutatesCountAccumulatesAcrossTwoMutations: Everquill Phoenix mutates
// twice onto the same bear. The second cast re-offers the mutate mode on the
// already-mutated pile and TimesMutated becomes 2.
func TestMutatesCountAccumulatesAcrossTwoMutations(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix1 := mustCorpusCard(t, reg, "Everquill Phoenix")
	phoenix2 := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, cfg := tokenReplGame(t, 303, phoenix1, phoenix2)
	first := moveSeededCard(t, e, 0, phoenix1, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRRR")
	mutateCastOnto(t, e, mutatedCastOption(t, e, first), bear, true)
	if got := e.G.Obj(bear).TimesMutated; got != 1 {
		t.Fatalf("after first mutate TimesMutated = %d, want 1", got)
	}

	second := moveSeededCard(t, e, 0, phoenix2, state.ZHand)
	addMana(t, e, 0, "RRRR")
	mutateCastOnto(t, e, mutatedCastOption(t, e, second), bear, true)

	pile := e.G.Obj(bear)
	if pile.TimesMutated != 2 {
		t.Fatalf("after second mutate TimesMutated = %d, want 2", pile.TimesMutated)
	}
	if len(pile.MergedCards) != 2 {
		t.Fatalf("merged under-cards = %d, want 2", len(pile.MergedCards))
	}
	if battlefieldNamedCount(t, e, "Feather") == 0 {
		t.Fatal("the second mutation did not fire the Mutates trigger")
	}
	replayCheck(t, e, cfg)
}

// TestMutateMergedCardsLeaveWithThePile: CR 702.140e -- when the mutated pile
// dies, EVERY card in it (the top card and each under-card) moves to the
// graveyard, and the pile marker is cleared.
func TestMutateMergedCardsLeaveWithThePile(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := tokenReplGame(t, 304, phoenix, bears)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := moveSeededCard(t, e, 0, bears, state.ZBattlefield)
	addMana(t, e, 0, "RRRR")
	mutateCastOnto(t, e, mutatedCastOption(t, e, phoenixID), bear, true)

	// On-top placement: the survivor object's top card is now the Phoenix, and
	// the demoted bear card is the single merged under-card with its own
	// parked object. Capture that under-card object before the pile leaves.
	under := e.G.Obj(bear).MergedCards[0].Obj
	e.emit(events.Sacrifice(bear))
	e.checkStateBased()
	for _, id := range []state.ObjID{bear, under} {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("card %d zone = %v, want graveyard (CR 702.140e)", id, func() string {
				if o == nil {
					return "nil"
				}
				return o.Zone.String()
			}())
		}
	}
	// The phoenix's card went to the graveyard as the pile's top card, so it
	// is present exactly once and never as an independent permanent.
	if battlefieldNamedCount(t, e, "Everquill Phoenix") != 0 {
		t.Fatal("the mutated Phoenix is still on the battlefield after the pile died")
	}
	if got := e.G.Obj(bear).TimesMutated; got != 0 {
		t.Fatalf("a departed pile still carries TimesMutated %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestMutateCostAndCoverage: the keyword is parsed to the printed cost (the
// mutate cost is paid instead of the mana cost) and the keyword/trigger pair
// no longer reports as unsupported.
func TestMutateCostAndCoverage(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	cost, ok := mutateCost(phoenix.Faces[0])
	if !ok {
		t.Fatal("Everquill Phoenix's K:Mutate:3 R did not parse")
	}
	if cost.Generic != 3 || cost.Colored[state.MR] != 1 {
		t.Fatalf("mutate cost = %+v, want 3 generic + 1 red", cost)
	}
	unsupported := reg.Unsupported(phoenix, effects.Supported())
	for _, u := range unsupported {
		if u == "kw:Mutate" || u == "trig:Mutates" {
			t.Fatalf("Everquill Phoenix still reports %q unsupported: %v", u, unsupported)
		}
	}
}

// TestMutateOfferRequiresNonHumanYouOwn: CR 702.140a's target restriction --
// a Human creature and an opponent's creature are both illegal, so with only
// those on the battlefield the mutate cast is withheld while the plain cast
// stays offered.
func TestMutateOfferRequiresNonHumanYouOwn(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, _ := tokenReplGame(t, 305, phoenix)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	putToken(t, e, 0, "Name:Test Human\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:1/1\nOracle:x\n", state.ZBattlefield)
	putToken(t, e, 1, mutateBearSrc, state.ZBattlefield) // an OPPONENT's non-Human creature
	addMana(t, e, 0, "RRRR")

	for _, o := range castOptions(t, e) {
		if o.Obj == phoenixID && o.Mode == "mutated" {
			t.Fatalf("mutate offered with no legal (non-Human, you-own) target: %+v", o)
		}
	}
	if o := plainCastOption(t, e, phoenixID); o.Obj == 0 {
		t.Fatal("the plain cast must remain offered")
	}
}
