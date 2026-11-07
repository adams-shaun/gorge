package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPrematureInstantCastWindow: the upkeep hold holds an instant-speed
// hand cast in the seat's own untap, upkeep and draw with an empty stack,
// and nothing else: not main 1, not the opponent's turn, not over a
// non-empty stack, not a non-hand (flashback) cast, not an exempt name, not
// a sorcery-speed card, not an activation.
func TestPrematureInstantCastWindow(t *testing.T) {
	cast := castSpell(0, 1)
	held := func(b Board, o decision.Option, fromHand, stackEmpty bool, exempt []string) bool {
		return b.PrematureInstantCast(o, fromHand, stackEmpty, exempt)
	}
	named := func(step state.Step, myTurn bool) Board {
		b := timingBoard(step, myTurn)
		b.Cards = TableOf(map[state.ObjID]Card{1: {PrintedName: "Opt", CMC: 1, InstantSpeed: true}})
		return b
	}
	for _, step := range []state.Step{state.StepUntap, state.StepUpkeep, state.StepDraw} {
		if !held(named(step, true), cast, true, true, nil) {
			t.Fatalf("step %s: an instant-speed hand cast on my turn with an empty stack is not held", step)
		}
	}
	up := named(state.StepUpkeep, true)
	// The precondition every false case below depends on: this exact board
	// and option hold, so each flip is the one fact changed.
	if !held(up, cast, true, true, []string{"Shock"}) {
		t.Fatal("precondition: the upkeep board with a non-matching exempt list is not held")
	}
	for _, tc := range []struct {
		name                 string
		b                    Board
		o                    decision.Option
		fromHand, stackEmpty bool
		exempt               []string
	}{
		{"main1", named(state.StepMain1, true), cast, true, true, nil},
		{"opponent's turn", named(state.StepUpkeep, false), cast, true, true, nil},
		{"non-empty stack", up, cast, true, false, nil},
		{"flashback origin", up, decision.Option{Index: 0, Kind: "cast", Obj: 1, Mode: "flashback"}, false, true, nil},
		{"exempt name", up, cast, true, true, []string{"Shock", "Opt"}},
		{"activation", up, decision.Option{Index: 0, Kind: "ability", Obj: 1}, true, true, nil},
	} {
		if held(tc.b, tc.o, tc.fromHand, tc.stackEmpty, tc.exempt) {
			t.Fatalf("%s: held, want not held", tc.name)
		}
	}
	sorcery := named(state.StepUpkeep, true)
	sorcery.Cards = TableOf(map[state.ObjID]Card{1: {PrintedName: "Opt", CMC: 1}})
	if held(sorcery, cast, true, true, nil) {
		t.Fatal("a sorcery-speed card is held")
	}
}
