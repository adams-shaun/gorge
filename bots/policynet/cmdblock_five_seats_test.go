package policynet

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestCommanderBlockFiveSeats: a five-seat view must not index past the
// four-seat block buffer when it looks up a damage source, and a damage
// source beyond the tracked blocks still lands under its relative seat
// column. The host accepts one to eight seats; only the first four have a
// block.
func TestCommanderBlockFiveSeats(t *testing.T) {
	v := view.View{Step: "declare-attackers"}
	for p := 0; p < 5; p++ {
		v.Players = append(v.Players, view.PlayerView{
			ID:         state.PlayerID(p),
			Commanders: []view.CardView{{ID: state.ObjID(100 + p), Name: "Commander"}},
		})
	}
	// Seat 2 has taken 7 from seat 4's commander. Relative seat 4 is the
	// second damage column of seat 2's block (k = 2), and seat 4 has no
	// block of its own.
	v.Players[2].CmdDamage = map[state.ObjID]int32{104: 7}
	if got := v.Players[2].CmdDamage[104]; got != 7 || v.Players[4].Commanders[0].ID != 104 {
		t.Fatalf("setup: damage %d from commander %d, want 7 from 104", got, v.Players[4].Commanders[0].ID)
	}

	st := EncodeState(v, 0)
	base := denseCmd0 + 2*commanderBlockWidth
	if got, want := st.Dense[base+denseCmdDamage0+1], float32(7)/21; got != want {
		t.Fatalf("seat 2's damage from relative seat 4 = %v, want %v", got, want)
	}
	if got := st.Dense[base+denseCmdDamage0]; got != 0 {
		t.Errorf("seat 2's relative seat 3 damage = %v, want 0", got)
	}

	// Only the fifth seat still has a commander: a tracked seat's damage from
	// an untracked source must still set the block (the commander game is
	// detected across the whole view, not only the four blocks).
	for p := 0; p < 4; p++ {
		v.Players[p].Commanders = nil
	}
	v.Players[1].CmdDamage = map[state.ObjID]int32{104: 7}
	st = EncodeState(v, 0)
	base = denseCmd0 + 1*commanderBlockWidth
	if got, want := st.Dense[base+denseCmdDamage0+2], float32(7)/21; got != want {
		t.Fatalf("seat 1's damage from the untracked relative seat 4 = %v, want %v", got, want)
	}
}
