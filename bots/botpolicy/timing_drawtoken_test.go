package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// drawTokenBoard builds the A6c board: one castable draw/token spell (card
// 10) in hand and a pool that pays its cost. Each test sets the turn, step
// and main facts the rule reads.
func drawTokenBoard(timing TimingRules, c Card) Board {
	c.Castable = true
	b := Board{Timing: timing, Cards: TableOf(map[state.ObjID]Card{10: c})}
	b.Pool[state.MU] = c.CMC
	return b
}

func drawTokenOptions() []decision.Option {
	return []decision.Option{
		{Index: 0, Kind: "cast", Obj: 10},
		{Index: 1, Kind: "pass"},
	}
}

// TestDrawSorceryWaitsForMain2: a sorcery draw is held in the seat's own
// first main and cast in the second, where the same decision is offered
// again. The rule off casts it in the first main.
func TestDrawSorceryWaitsForMain2(t *testing.T) {
	card := Card{Draw: true, CMC: 3, ManaCost: "2 U"}
	for _, tc := range []struct {
		name      string
		step      state.Step
		firstMain bool
		want      int
	}{
		{name: "first main", step: state.StepMain1, firstMain: true, want: 1},
		{name: "second main", step: state.StepMain2, firstMain: false, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			build := func(timing TimingRules) Board {
				b := drawTokenBoard(timing, card)
				b.IsMain, b.MyTurn, b.Step, b.FirstMain = true, true, tc.step, tc.firstMain
				return b
			}
			if got, _ := castDecision(build(TimingRules{}), drawTokenOptions()); got != 0 {
				t.Fatalf("rule off: chose option %d, want the draw cast (0)", got)
			}
			if got, _ := castDecision(build(TimingRules{DrawTokenLate: true}), drawTokenOptions()); got != tc.want {
				t.Fatalf("rule on: chose option %d, want option %d", got, tc.want)
			}
		})
	}
}

// TestTokenSorceryWaitsForMain2 is the token half of the sorcery rule.
func TestTokenSorceryWaitsForMain2(t *testing.T) {
	card := Card{Token: true, CMC: 3, ManaCost: "2 U"}
	for _, tc := range []struct {
		name      string
		step      state.Step
		firstMain bool
		want      int
	}{
		{name: "first main", step: state.StepMain1, firstMain: true, want: 1},
		{name: "second main", step: state.StepMain2, firstMain: false, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			build := func(timing TimingRules) Board {
				b := drawTokenBoard(timing, card)
				b.IsMain, b.MyTurn, b.Step, b.FirstMain = true, true, tc.step, tc.firstMain
				return b
			}
			if got, _ := castDecision(build(TimingRules{}), drawTokenOptions()); got != 0 {
				t.Fatalf("rule off: chose option %d, want the token spell cast (0)", got)
			}
			if got, _ := castDecision(build(TimingRules{DrawTokenLate: true}), drawTokenOptions()); got != tc.want {
				t.Fatalf("rule on: chose option %d, want option %d", got, tc.want)
			}
		})
	}
}

// TestInstantDrawWaitsForEndStep: an instant draw is held through an
// opponent's upkeep, fires at that opponent's end step, and fires early
// when the hand is nearly empty.
func TestInstantDrawWaitsForEndStep(t *testing.T) {
	card := Card{Draw: true, InstantSpeed: true, CMC: 1, ManaCost: "U"}
	for _, tc := range []struct {
		name     string
		step     state.Step
		handSize int32
		want     int
	}{
		{name: "opponent upkeep", step: state.StepUpkeep, handSize: 3, want: 1},
		{name: "opponent end step", step: state.StepEnd, handSize: 3, want: 0},
		{name: "nearly empty hand", step: state.StepUpkeep, handSize: 1, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			build := func(timing TimingRules) Board {
				b := drawTokenBoard(timing, card)
				b.MyTurn, b.Step, b.HandSize = false, tc.step, tc.handSize
				return b
			}
			if got, _ := castDecision(build(TimingRules{}), drawTokenOptions()); got != 0 {
				t.Fatalf("rule off: chose option %d, want the draw cast (0)", got)
			}
			if got, _ := castDecision(build(TimingRules{DrawTokenLate: true}), drawTokenOptions()); got != tc.want {
				t.Fatalf("rule on: chose option %d, want option %d", got, tc.want)
			}
		})
	}
}

// TestInstantTokenWaitsForAttack: an instant token is held through an
// opponent's upkeep and fires from that opponent's declare-attackers step.
func TestInstantTokenWaitsForAttack(t *testing.T) {
	card := Card{Token: true, InstantSpeed: true, CMC: 1, ManaCost: "U"}
	for _, tc := range []struct {
		name string
		step state.Step
		want int
	}{
		{name: "opponent upkeep", step: state.StepUpkeep, want: 1},
		{name: "opponent declare attackers", step: state.StepDeclareAttackers, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			build := func(timing TimingRules) Board {
				b := drawTokenBoard(timing, card)
				b.MyTurn, b.Step = false, tc.step
				return b
			}
			if got, _ := castDecision(build(TimingRules{}), drawTokenOptions()); got != 0 {
				t.Fatalf("rule off: chose option %d, want the token spell cast (0)", got)
			}
			if got, _ := castDecision(build(TimingRules{DrawTokenLate: true}), drawTokenOptions()); got != tc.want {
				t.Fatalf("rule on: chose option %d, want option %d", got, tc.want)
			}
		})
	}
}
