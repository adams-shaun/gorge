package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestConverterGateNeverTakesAChoiceConverter: Skyshroud Elf's "{1}: Add {R}
// or {W}" is a converter whose colour is a script-level choice. The costed
// colour ask is never steered (manaOptionColour refuses a "Pay 1: Add R"
// label), so the engine's fallback adds the first colour, red. Priced as the
// card's most-needed colour (white) the gate saw progress, activated it, paid
// the floating {R} and added {R} back, forever (mtgsim cmdr4 seed 89884, 4
// seats, until the livelock watcher fired). A choice converter is no progress.
func TestConverterGateNeverTakesAChoiceConverter(t *testing.T) {
	elf := cards.ManaProduction{Any: true, Colour: [6]int32{1, 0, 0, 1, 1, 0}}
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Label: "Activate Skyshroud Elf for mana", Obj: 7, Cost: "1"},
			{Index: 1, Kind: "pass", Label: "Pass priority"},
		}}
	b := Board{IsMain: true,
		Cards: TableOf(map[state.ObjID]Card{
			3: {CMC: 1, ManaCost: "W", Castable: true},
			7: {Produces: elf},
		}),
		Pool: state.Mana{0, 0, 0, 1, 0, 0}}
	if in := Decide(b, &d, rng(1)); d.Options[in.Choices[0]].Kind != "pass" {
		t.Fatalf("choice converter over a floating {R} = %+v, want the pass", in)
	}
}
