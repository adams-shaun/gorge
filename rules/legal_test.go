package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func kinds(opts []decision.Option) map[string]int {
	m := map[string]int{}
	for _, o := range opts {
		m[o.Kind]++
	}
	return m
}

// handEngine puts an exact set of cards in seat 0's hand at main phase, so
// option enumeration can be asserted precisely.
func handEngine(t *testing.T, hand ...*cards.Card) *Engine {
	t.Helper()
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	var ids []state.ObjID
	for _, c := range hand {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZHand
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZHand, 0, ids)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

func TestLandIsPlayableOnceAtSorcerySpeed(t *testing.T) {
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	e := handEngine(t, mtn, mtn)
	opts := e.legalActions(0)
	if kinds(opts)["play_land"] != 2 {
		t.Fatalf("two lands in hand should yield two options: %v", kinds(opts))
	}
	e.G.Players[0].LandsPlayed = 1
	if kinds(e.legalActions(0))["play_land"] != 0 {
		t.Error("land drop already used, no land option should appear")
	}
	e.G.Players[0].LandsPlayed = 0
	e.G.Step = state.StepUpkeep
	if kinds(e.legalActions(0))["play_land"] != 0 {
		t.Error("lands are sorcery-speed only")
	}
}

func TestCastRequiresPayableManaAndRightTiming(t *testing.T) {
	bolt := card(t, "Name:Lightning Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	bear := card(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, bolt, bear)

	if kinds(e.legalActions(0))["cast"] != 0 {
		t.Fatal("nothing is castable with an empty pool")
	}
	e.G.Players[0].Pool[state.MR] = 1
	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("R in pool should make Bolt castable and Bear not")
	}
	e.G.Players[0].Pool[state.MG] = 2
	if got := kinds(e.legalActions(0))["cast"]; got != 2 {
		t.Fatalf("cast options = %d, want 2", got)
	}
	// Off-turn, only the instant may be cast.
	e.G.Active = 1
	if got := kinds(e.legalActions(0))["cast"]; got != 1 {
		t.Fatalf("off-turn cast options = %d, want 1 (the instant)", got)
	}
}

func TestFlashCreatureIsCastableOffTurn(t *testing.T) {
	flash := card(t, "Name:Flashy\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nK:Flash\nOracle:x\n")
	e := handEngine(t, flash)
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Active = 1
	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("Flash should allow casting on another player's turn")
	}
}

// TestFlashGrantedByAContinuousEffectIsCastableOffTurn is the Ruling T19-c
// regression test: legalActions' instant-speed check must read the keyword
// through the layer system (Engine.HasKeyword), not the printed face
// (f.HasKeyword), so a creature granted Flash by another permanent's
// continuous effect -- rather than printed on its own card -- is castable
// off turn too.
func TestFlashGrantedByAContinuousEffectIsCastableOffTurn(t *testing.T) {
	bear := card(t, "Name:Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, bear)
	e.G.Players[0].Pool[state.MG] = 1
	e.G.Active = 1

	granter := card(t, "Name:Granter\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n")
	granterObj := e.G.AddObject(granter, 0)
	granterObj.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{granterObj.ID})
	e.AddContinuous(ContinuousEffect{Source: granterObj.ID, Timestamp: 1, Layer: LAbilities,
		Affects: "Creature.YouCtrl", Controller: 0, AddKeywords: []string{"Flash"}})

	handID := e.G.Zone(state.ZHand, 0)[0]
	if !e.HasKeyword(handID, "Flash") {
		t.Fatal("the hand creature should see the granted Flash through the layer system")
	}
	if kinds(e.legalActions(0))["cast"] != 1 {
		t.Fatal("a creature granted Flash by a continuous effect should be castable off turn")
	}
}

func TestUntappedLandOffersItsManaAbility(t *testing.T) {
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	e := handEngine(t)
	o := e.G.AddObject(mtn, 0)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	if kinds(e.legalActions(0))["activate"] != 1 {
		t.Fatal("an untapped Mountain should offer its mana ability")
	}
	o.Tapped = true
	if kinds(e.legalActions(0))["activate"] != 0 {
		t.Error("a tapped land offers nothing")
	}
}

func TestPassIsAlwaysOffered(t *testing.T) {
	e := handEngine(t)
	if kinds(e.legalActions(0))["pass"] != 1 {
		t.Fatal("pass must always be available")
	}
	if e.legalActions(0)[len(e.legalActions(0))-1].Kind != "pass" {
		t.Error("pass should be the last option so clients can default to it")
	}
}

func TestOptionIndicesAreContiguous(t *testing.T) {
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	e := handEngine(t, mtn, mtn, mtn)
	opts := e.legalActions(0)
	for i, o := range opts {
		if o.Index != i {
			t.Fatalf("option %d has Index %d", i, o.Index)
		}
	}
}
