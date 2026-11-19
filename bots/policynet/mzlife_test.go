package policynet

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

func mzHas(sk *sink, tok string) bool {
	r := hashID(tok)
	for _, f := range sk.fs {
		if f.Row == r {
			return true
		}
	}
	return false
}

func mzLifeView(viewer state.PlayerID, lost map[state.PlayerID]bool, life []int32) view.View {
	v := view.View{Viewer: viewer}
	for i, l := range life {
		v.Players = append(v.Players, view.PlayerView{ID: state.PlayerID(i), Life: l, Lost: lost[state.PlayerID(i)]})
	}
	return v
}

// A 4-seat pod where seat 1 has lost: the viewer (seat 2) reads seat 3's life
// in the first opponent row, then seat 0, and the dead seat's non-positive life
// never turns lethal on.
func TestMZLifeRowsSkipLostSeat(t *testing.T) {
	v := mzLifeView(2, map[state.PlayerID]bool{1: true}, []int32{30, -3, 40, 20})
	sk := &sink{}
	mzState(v, 2, sk)
	if !mzHas(sk, "mz|opp|life=10") {
		t.Fatal("first opponent row is not seat 3's life (20 -> bucket 10)")
	}
	if !mzHas(sk, "mz|opp1|life=15") {
		t.Fatal("second opponent row is not seat 0's life (30 -> bucket 15)")
	}
	if mzHas(sk, "mz|opp2|life=0") || mzHas(sk, "mz|opp|life=0") {
		t.Fatal("the lost seat's life was read")
	}
	if mzHas(sk, "mz|me|lethal-unblocked") || mzHas(sk, "mz|opp|lethal-unblocked") {
		t.Fatal("lethal on with an empty board: a dead seat's life <= 0 was read")
	}
	if !mzHas(sk, "mz|lifediff=13") { // (40-20)/3+7
		t.Fatal("first lifediff row is not against seat 3")
	}
}

// Opponents' power is never summed against the viewer's life.
func TestMZLethalIsPerOpponent(t *testing.T) {
	v := mzLifeView(0, nil, []int32{5, 40, 40, 40})
	for p := 1; p <= 3; p++ {
		v.Players[p].Battlefield = []view.CardView{{Name: "Bear", Types: "Creature", Power: 2, Toughness: 2}}
	}
	sk := &sink{}
	mzState(v, 0, sk)
	if mzHas(sk, "mz|opp|lethal-unblocked") {
		t.Fatal("three 2-power opponents summed into lethal against 5 life")
	}
	v.Players[2].Battlefield[0].Power = 5
	sk = &sink{}
	mzState(v, 0, sk)
	if !mzHas(sk, "mz|opp|lethal-unblocked") {
		t.Fatal("one opponent with 5 power did not read as lethal")
	}
}
