package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// feeFilterBoard is boundBoard with the ETB triggers the board's own cards
// raise answered: Mardu Devotee's "when this creature enters, scry 2" asks a
// KArrange before the first main phase can be reached.
func feeFilterBoard(t *testing.T, board, hand []string) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	for _, n := range hand {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	for _, n := range hand {
		findAndMoveToHand(t, e, 0, n)
	}
	e.pending = nil
	e.Advance()
	// Answer the board's own ETB asks (the Devotee's scry) up to priority.
	for i := 0; i < 60; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while settling the board's ETB triggers")
		}
		if d.Kind == decision.KPriority {
			break
		}
		if d.Kind != decision.KArrange {
			t.Fatalf("unexpected %s decision while settling the board's ETB triggers: %+v", d.Kind, d)
		}
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		if d.Max >= 0 && int(d.Max) < len(ch) {
			ch = ch[:d.Max]
		}
		submitChoices(t, e, ch...)
	}
	// driveToStep cannot cross the Devotee's scry ask, so this drive answers
	// KArrange and passes priority until seat 0's first main of turn 3.
	for i := 0; i < 4000; i++ {
		if e.G.Turn == 3 && e.G.Active == 0 && e.G.Step == state.StepMain1 {
			break
		}
		if e.G.Over {
			t.Fatalf("game ended before turn 3 seat 0 main1")
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending while driving to turn 3 seat 0 main1")
		}
		if d.Kind == decision.KArrange {
			ch := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				ch = append(ch, o.Index)
			}
			if d.Max >= 0 && int(d.Max) < len(ch) {
				ch = ch[:d.Max]
			}
			submitChoices(t, e, ch...)
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %s decision while driving to turn 3 seat 0 main1: %+v", d.Kind, d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		submitChoices(t, e, idx)
	}
	if e.G.Turn != 3 || e.G.Active != 0 || e.G.Step != state.StepMain1 {
		t.Fatalf("did not reach turn 3 seat 0 main1")
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		e.G.Obj(id).SummonSick = false
	}
	return e
}

// TestFeeFilterSourceFundsCastOffer is the regression for Mardu Devotee's
// "{1}: Add {R}, {W}, or {B}": a mana ability whose OWN cost is mana paid from
// another source must still count toward the potential-action offer census
// and the planner's payment witness.
//
// The board is two untapped Plains and a Mardu Devotee against Caustic Exhale
// ({B}, plus "behold a Dragon or pay {1}") in hand. Nothing is floated, so the
// cast is reachable only through the potential bound: one Plains pays the
// Devotee's {1} for {B}, the other Plains pays the spell's own {1}. The cast
// must be offered, and the payment planner (whose relaxed census proof covers
// the fee-only source) must not call it unpayable.
func TestFeeFilterSourceFundsCastOffer(t *testing.T) {
	e := feeFilterBoard(t,
		[]string{"Plains", "Plains", "Mardu Devotee"},
		[]string{"Caustic Exhale"})

	// Preconditions: the Devotee's fee ability is activatable and its cost is
	// bare generic mana (no tap), the fee is payable from one Plains, and
	// nothing is floating yet.
	dev := boundBattlefieldID(t, e, "Mardu Devotee")
	ma := boundManaAbility(t, e, dev)
	if c := e.abilityCompiledCost(ma).Cost; c.Generic != 1 || c.Tap {
		t.Fatalf("precondition: Mardu Devotee's mana ability cost = %+v, want a bare {1}", c)
	}
	if e.G.Players[0].Pool != (state.Mana{}) {
		t.Fatalf("precondition: seat 0 already floats %v", e.G.Players[0].Pool)
	}
	var onePlains state.Mana
	onePlains[state.MW] = 1
	if !e.manaAbilityPayablePool(0, dev, ma, &onePlains) {
		t.Fatal("precondition: Mardu Devotee's {1} is not payable from one Plains' {W}")
	}
	exhale := boundHandID(t, e, "Caustic Exhale")

	// The potential bound must count the Devotee's {B}, ...
	bound, _ := e.potentialWalkOf(0, true)
	if bound[state.MB] == 0 {
		t.Fatalf("potential bound %v does not contain the Devotee's {B}", bound)
	}
	// ... the potential walk must offer the cast, ...
	boundPlayOption(t, e, exhale)
	// ... and the planner must not prove it unpayable: the relaxed census
	// proof has to price the Devotee's own production into the witness it
	// confirms against the offer gate.
	listed := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != exhale {
			continue
		}
		listed = true
		if pp.Reason == "insufficient" {
			t.Fatalf("the cast is offered by the potential walk but the planner proves it unpayable: %+v", pp)
		}
	}
	if !listed {
		t.Fatal("the potential payment plans do not list the cast at all")
	}
}
