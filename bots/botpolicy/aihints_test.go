package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAIHintsPlayMain1CastsPrecombat: with the rule armed, a PlayMain1 card
// earns the precombat bonus and clears a C9 threshold that holds it without
// the hint. The card is a 2/2 (base score 38); the threshold sits one point
// above that base, so only the +40 hint can clear it.
func TestAIHintsPlayMain1CastsPrecombat(t *testing.T) {
	card := Card{Creature: true, Power: 2, Castable: true, PlayMain1: cards.PlayMain1Always}
	b := Board{IsMain: true, FirstMain: true, MyTurn: true,
		Cards: TableOf(map[state.ObjID]Card{1: card})}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{castCreature(0, 1)}}
	tuned := withTuned(b, func(w *CastWeights) { w.CastThreshold = 39 })

	if got := tuned.chooseCast(d); got != -1 {
		t.Fatalf("hint off = option %d, want -1: the 2/2's 38 is below the 39 threshold", got)
	}
	armed := tuned
	armed.AIHints = AIHints{Enabled: true}
	if got := armed.chooseCast(d); got != 0 {
		t.Fatalf("hint on = option %d, want 0: the PlayMain1 bonus clears the threshold", got)
	}
	// The bonus is the FIRST main phase's: outside it the hint changes
	// nothing, so the threshold still holds the cast.
	armed.FirstMain = false
	if got := armed.chooseCast(d); got != -1 {
		t.Fatalf("hint on outside the first main = option %d, want -1", got)
	}
	// An opponent's first main is not the caster's precombat window: the
	// bonus must not fire with MyTurn false even though FirstMain is set.
	armed.FirstMain = true
	armed.MyTurn = false
	if got := armed.chooseCast(d); got != -1 {
		t.Fatalf("hint on in the opponent's first main = option %d, want -1", got)
	}
}

// TestAIHintsPlayMain1Conditions pins the closed PlayMain1 vocabulary: ALWAYS
// is unconditional, OPPONENTCREATURES reads the opponent count, and TRUE (or
// an unknown spelling) reads the caster's own creature count.
func TestAIHintsPlayMain1Conditions(t *testing.T) {
	b := Board{AIHints: AIHints{Enabled: true}}
	cases := []struct {
		mode      string
		own, opp  int32
		wantBonus bool
	}{
		{cards.PlayMain1Always, 0, 0, true},
		{cards.PlayMain1OwnCreatures, 0, 3, false},
		{cards.PlayMain1OwnCreatures, 1, 0, true},
		{cards.PlayMain1OpponentCreatures, 2, 0, false},
		{cards.PlayMain1OpponentCreatures, 0, 1, true},
		{"", 1, 1, false},
	}
	for _, tc := range cases {
		got := b.aiHintsCastBonus(Card{PlayMain1: tc.mode}, true, tc.own, tc.opp) > 0
		if got != tc.wantBonus {
			t.Errorf("mode %q own=%d opp=%d bonus=%v, want %v", tc.mode, tc.own, tc.opp, got, tc.wantBonus)
		}
	}
}

// TestAIHintsRemoveDeckNeverCast: a RemoveDeck card is never chosen while the
// rule is armed, even when it outranks the alternative; without the rule the
// ordinary ranking still picks it. This is the deck-wide-bad shape the hint
// names -- the AI does not know how to play it.
func TestAIHintsRemoveDeckNeverCast(t *testing.T) {
	table := map[state.ObjID]Card{
		1: {Creature: true, Power: 3, Castable: true, RemoveDeck: true},
		2: {Creature: true, Power: 1, Castable: true},
	}
	b := Board{IsMain: true, Cards: TableOf(table)}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{castCreature(0, 1), castCreature(1, 2)}}
	if got := b.chooseCast(d); got != 0 {
		t.Fatalf("hint off = option %d, want 0: the 3-power creature outranks the 1/1", got)
	}
	armed := b
	armed.AIHints = AIHints{Enabled: true}
	if got := armed.chooseCast(d); got != 1 {
		t.Fatalf("hint on = option %d, want 1: the RemoveDeck card is vetoed", got)
	}
	only := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{castCreature(0, 1)}}
	if got := armed.chooseCast(only); got != -1 {
		t.Fatalf("hint on with only the RemoveDeck card = option %d, want -1", got)
	}
}

// TestAIHintsRemoveDeckNeverTapTarget: the armed veto must reach the tap gate
// too. chooseCast refuses the RemoveDeck card, so floating mana toward it
// wastes the tap (and defeats the auto-pay adapter's waste protection through
// TapIntent/AnyCastableNow). With the rule off the same board still taps.
func TestAIHintsRemoveDeckNeverTapTarget(t *testing.T) {
	var island cards.ManaProduction
	island.Colour[state.MU] = 1
	brd := Board{IsMain: true, FirstMain: true, MyTurn: true,
		Cards: TableOf(map[state.ObjID]Card{
			1: {Castable: true, CMC: 1, RemoveDeck: true},
			7: {OnBattlefield: true, Basic: true, Produces: island},
		}),
	}
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Obj: 7},
			{Index: 1, Kind: "pass"},
		}}

	// Preconditions: the card really is unpayable, and the gate really does
	// want a tap for it with the rule off.
	if brd.poolPays(1, brd.Cards.Get(1)) {
		t.Fatalf("precondition: poolPays(1) = true, want the card unpayable")
	}
	if !brd.tapWants(d) {
		t.Fatalf("precondition: tapWants = false with the rule off, want a tap")
	}
	if got := brd.chooseTap(d); got != 0 {
		t.Fatalf("hint off chooseTap = option %d, want 0 (tap the island)", got)
	}
	if !brd.AnyCastableNow(0, d) {
		t.Fatalf("hint off AnyCastableNow = false, want true")
	}

	armed := brd
	armed.AIHints = AIHints{Enabled: true}
	if armed.tapWants(d) {
		t.Fatalf("hint on tapWants = true, want false: the RemoveDeck card is vetoed")
	}
	if got := armed.chooseTap(d); got != -1 {
		t.Fatalf("hint on chooseTap = option %d, want -1", got)
	}
	if armed.AnyCastableNow(0, d) {
		t.Fatalf("hint on AnyCastableNow = true, want false: the only intent is vetoed")
	}
}
