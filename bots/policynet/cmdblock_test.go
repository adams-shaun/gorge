package policynet

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// commanderBlockView is a four-seat Commander position: seat 2 attacks with a
// partner pair (objects 102 and 112), seat 1 has taken 7 from 102 and 11 from
// 112, and seat 2's pair is on the battlefield having been cast twice in
// total. Seat 3 has left the game.
func commanderBlockView() view.View {
	v := view.View{Step: "declare-attackers"}
	for p := 0; p < 4; p++ {
		id := state.ObjID(100 + p)
		pv := view.PlayerView{
			ID:             state.PlayerID(p),
			Commanders:     []view.CardView{{ID: id, Name: "Commander"}},
			CommanderCasts: []int32{0},
		}
		v.Players = append(v.Players, pv)
	}
	v.Players[0].Commanders[0].ID = 100
	v.Players[1].Commanders[0].ID = 101
	v.Players[2].Commanders = []view.CardView{
		{ID: 102, Name: "Commander A"},
		{ID: 112, Name: "Commander B"},
	}
	v.Players[2].CommanderCasts = []int32{1, 1}
	v.Players[2].Battlefield = []view.CardView{
		{ID: 102, Name: "Commander A", Types: "Creature"},
		{ID: 112, Name: "Commander B", Types: "Creature"},
	}
	v.Players[1].CmdDamage = map[state.ObjID]int32{102: 7, 112: 11}
	v.Players[3].Lost = true
	return v
}

// TestCommanderBlockTracksDamageAndTax pins the Commander block against a
// four-seat view: the defender's block carries the max partner tally under
// the attacker's relative seat, the attacker's cast column moves on a recast
// (partner casts sum), the battlefield bit follows the commander, and the
// left-the-game bit is always present.
func TestCommanderBlockTracksDamageAndTax(t *testing.T) {
	v := commanderBlockView()
	// Preconditions: the looked-up values are nonzero and the two partner
	// tallies differ, so the max is observable.
	if v.Players[1].CmdDamage[102] == 0 || v.Players[1].CmdDamage[102] == v.Players[1].CmdDamage[112] {
		t.Fatalf("setup: partner tallies %v do not distinguish the max", v.Players[1].CmdDamage)
	}
	if got := len(v.Players[2].Commanders); got != 2 || len(v.Players[2].CommanderCasts) != 2 {
		t.Fatalf("setup: attacker has %d commanders and %d cast counts", got, len(v.Players[2].CommanderCasts))
	}

	st := EncodeState(v, 0)
	// Relative seat 1 is the defender; relative seat 2 (its next seat) is the
	// attacker, so the defender's first damage column is the attacker's.
	defBase := denseCmd0 + 1*commanderBlockWidth
	if got, want := st.Dense[defBase+denseCmdDamage0], float32(11)/21; got != want {
		t.Fatalf("defender's damage from the attacker = %v, want max(7,11)/21 = %v", got, want)
	}
	for k := 1; k <= 3; k++ {
		want := float32(0)
		if k == 1 {
			want = float32(11) / 21 // the attacker's relative seat
		}
		if got := st.Dense[defBase+denseCmdDamage0+k-1]; got != want {
			t.Errorf("defender's damage column %d = %v, want %v", k, got, want)
		}
	}

	atkBase := denseCmd0 + 2*commanderBlockWidth
	if got, want := st.Dense[atkBase+denseCmdCasts], float32(2)/4; got != want {
		t.Fatalf("attacker's cast column = %v, want partner-sum 2/4 = %v", got, want)
	}
	if got := st.Dense[atkBase+denseCmdOnField]; got != 1 {
		t.Fatalf("attacker's battlefield bit = %v, want 1 (both partners on the battlefield)", got)
	}

	// A recast moves the tax column.
	v.Players[2].CommanderCasts = []int32{2, 1}
	st = EncodeState(v, 0)
	if got, want := st.Dense[atkBase+denseCmdCasts], float32(3)/4; got != want {
		t.Fatalf("after a recast the attacker's cast column = %v, want 3/4 = %v", got, want)
	}

	// The battlefield bit follows the commander off the battlefield.
	v.Players[2].Battlefield = nil
	st = EncodeState(v, 0)
	if got := st.Dense[atkBase+denseCmdOnField]; got != 0 {
		t.Fatalf("after the commanders leave the battlefield the bit = %v, want 0", got)
	}

	// The left-the-game bit is present even outside the commander data.
	if got := st.Dense[denseCmd0+3*commanderBlockWidth+denseCmdLost]; got != 1 {
		t.Fatalf("seat 3's left-the-game bit = %v, want 1", got)
	}

	// A non-Commander game fills the block with zeros except left-the-game.
	v = commanderBlockView()
	for p := range v.Players {
		v.Players[p].Commanders = nil
		v.Players[p].CommanderCasts = nil
		v.Players[p].CmdDamage = nil
		v.Players[p].Battlefield = nil
	}
	st = EncodeState(v, 0)
	for blk := 0; blk < maxSeatBlocks; blk++ {
		base := denseCmd0 + blk*commanderBlockWidth
		for j := 0; j < commanderBlockWidth; j++ {
			want := float32(0)
			if j == denseCmdLost && blk == 3 {
				want = 1
			}
			if got := st.Dense[base+j]; got != want {
				t.Errorf("non-Commander block %d value %d = %v, want %v", blk, j, got, want)
			}
		}
	}
}

// TestDenseWidth68CheckpointUnchanged: a policynet Model checkpoint written
// before the Commander block (state dense width 68, current 28-wide option
// row) loads, keeps reading the first 68 dense columns, and scores a state
// with the new block set bit-identically.
func TestDenseWidth68CheckpointUnchanged(t *testing.T) {
	m := NewModel(TableRows, 4, 6, rand.New(rand.NewPCG(7, 8)))
	m.stateDenseW = DenseWidthV1
	m.StateW = m.StateW[:DenseWidthV1*m.H]
	m.Features = FeaturesMZ
	if m.StateDenseW() != DenseWidthV1 || len(m.StateW) != DenseWidthV1*m.H {
		t.Fatalf("setup: state width %d, weights %d", m.StateDenseW(), len(m.StateW))
	}

	st := State{Dense: make([]float32, DenseWidth), Sparse: []Feature{{Row: 9, Value: 1}}}
	for i := range st.Dense {
		st.Dense[i] = float32(i%5) / 5
	}
	commanderNonZero := false
	for _, x := range st.Dense[DenseWidthV1:] {
		if x != 0 {
			commanderNonZero = true
		}
	}
	if !commanderNonZero {
		t.Fatal("fixture has no non-zero Commander block; the comparison proves nothing")
	}
	opts := []Option{{Hashed: []Feature{{Row: 12, Value: 1}}, Slots: []Feature{{Row: 3, Value: 1}}, Dense: make([]float32, OptionDenseWidth)}}
	want := m.Score(st, opts)

	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint64(buf.Bytes()[8:16]); got != EncoderHashForV1Dense(FeaturesMZ) {
		t.Fatalf("68-wide checkpoint hash %#x, want %#x", got, EncoderHashForV1Dense(FeaturesMZ))
	}
	back, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("68-wide LoadCheckpoint: %v", err)
	}
	if back.StateDenseW() != DenseWidthV1 || len(back.StateW) != DenseWidthV1*back.H {
		t.Fatalf("loaded state width %d, weights %d", back.StateDenseW(), len(back.StateW))
	}
	zeroed := st
	zeroed.Dense = append([]float32(nil), st.Dense...)
	clear(zeroed.Dense[DenseWidthV1:])
	for _, tc := range []struct {
		name string
		s    State
	}{{"full", st}, {"zeroed", zeroed}} {
		got := back.Score(tc.s, opts)
		for i := range want {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Errorf("%s: score %d %v vs %v", tc.name, i, got[i], want[i])
			}
		}
	}
}
