package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestChooseLandAlreadyAvailableConsidered is the L1 "which colours are
// already available" side of the bl1 land-drop fix. The hand wants {U}{G};
// a blue source is already on the battlefield (OnBattlefield), so blue is
// covered and the still-missing colour is green. Both candidate lands are
// basics, so the blind basic-first rule plays whatever basic is offered
// first -- the redundant Island (obj 10), index 0 -- and the hand never gets
// its green. The colour-aware drop subtracts the battlefield's blue from the
// need, leaving green as the only unmet colour, and plays the Forest (obj
// 11). Index 1 is blue (U); index 4 green (G).
func TestChooseLandAlreadyAvailableConsidered(t *testing.T) {
	b := priorityCards(map[state.ObjID]Card{
		1:  {CMC: 2, ManaCost: "U G", Castable: true},          // a {U}{G} spell in hand
		20: {OnBattlefield: true, Produces: prod(state.MU, 1)}, // a blue source already in play
		10: {Basic: true, Produces: prod(state.MU, 1)},         // an Island, a redundant basic offered first
		11: {Basic: true, Produces: prod(state.MG, 1)},         // a Forest, the missing green
	})
	got, d := castDecision(b, []decision.Option{
		playLand(0, 10), // the redundant Island, listed first
		playLand(1, 11), // the Forest, the colour actually missing
	})
	if d.Options[got].Obj != 11 {
		t.Fatalf("land drop = obj %d (option %d), want the Forest (obj 11): blue is already available, green is what the hand still needs", d.Options[got].Obj, got)
	}
}

// TestChooseLandNoOptionsReturnsMinusOne pins that a land drop with no
// offered land still returns -1 so the cast branch is reached, and a
// factless offered land (no census entry) still ranks deterministically (as
// zero coverage/nonbasic/flex 0) without panicking -- the C5-shaped safety
// for the land branch.
func TestChooseLandNoOptionsReturnsMinusOne(t *testing.T) {
	b := priorityCards(nil)
	if got := b.chooseLand(&decision.Decision{Player: 0, Kind: decision.KPriority, Options: nil}); got != -1 {
		t.Fatalf("chooseLand with no offer = %d, want -1", got)
	}
	// A land whose Obj is not in the census still picks (index 0) rather than
	// panicking: zero facts read as zero coverage, nonbasic, flex 0.
	if got := b.chooseLand(&decision.Decision{Player: 0, Kind: decision.KPriority,
		Options: []decision.Option{{Index: 0, Kind: "play_land", Obj: 999}}}); got != 0 {
		t.Fatalf("chooseLand with a factless land = %d, want 0 (the only offered index)", got)
	}
}
