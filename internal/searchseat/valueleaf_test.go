package searchseat

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// Options.Value turns into the teacher's Leaf only for a model with a value
// head, and the Leaf is exactly the value head on the encoded leaf view.
func TestValueLeafIsTheValueHead(t *testing.T) {
	if valueLeaf(nil) != nil {
		t.Fatal("nil model must keep the heuristic leaf")
	}
	m := policynet.NewModel(policynet.TableRows, 8, 6, rand.New(rand.NewPCG(1, 2)))
	if valueLeaf(m) != nil {
		t.Fatal("a model without a value head must keep the heuristic leaf")
	}
	m.InitValue(5, rand.New(rand.NewPCG(3, 4)))
	leaf := valueLeaf(m)
	if leaf == nil {
		t.Fatal("value model gave no leaf")
	}
	v := view.View{Turn: 3, Players: []view.PlayerView{{ID: 0, Life: 17}, {ID: 1, Life: 9}}}
	for _, actor := range []state.PlayerID{0, 1} {
		want := float64(m.Value(policynet.EncodeState(v, actor)))
		if got := leaf(v, actor); got != want || got <= 0 || got >= 1 {
			t.Fatalf("actor %d: leaf %v, value head %v", actor, got, want)
		}
	}
}
