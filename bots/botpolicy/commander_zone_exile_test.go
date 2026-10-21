package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g49: a commander in exile that still holds a cast-from-exile
// permission (the engine names it on the "leave" option's Mode:
// adventure_recast, foretell_cast, plot_cast) is left in exile. Casting it
// from there is untaxed (CR 903.8 taxes only command-zone casts); the
// command zone ends the permission and charges the tax.
func TestCommanderZoneLeavesExileWithCastPermission(t *testing.T) {
	fresh := Board{
		Cards:      TableOf(map[state.ObjID]Card{100: {Creature: true, Power: 4, CMC: 4}}),
		Commanders: TableOf(map[state.ObjID]Commander{100: {Casts: 0}}),
	}
	for _, mode := range []string{"adventure_recast", "foretell_cast", "plot_cast"} {
		d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KCommanderZone, Min: 1, Max: 1,
			Options: []decision.Option{
				{Index: 0, Kind: "command_zone", Obj: 100},
				{Index: 1, Kind: "leave", Obj: 100, Mode: mode},
			}}
		if in := Decide(fresh, d, rng(1)); len(in.Choices) != 1 || in.Choices[0] != 1 {
			t.Errorf("%s: fresh commander = %v, want leave (option 1): the exile cast is untaxed", mode, in.Choices)
		}
	}
	// Guard: a plain "leave" (no permission) keeps the tax-aware rule, so a
	// fresh commander still goes to the command zone.
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KCommanderZone, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "command_zone", Obj: 100},
			{Index: 1, Kind: "leave", Obj: 100},
		}}
	if in := Decide(fresh, d, rng(1)); len(in.Choices) != 1 || in.Choices[0] != 0 {
		t.Fatalf("plain leave: fresh commander = %v, want the command zone (option 0)", in.Choices)
	}
}
