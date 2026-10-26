package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7: exiledBySource ignores a link made by a previous incarnation.
func TestExiledBySourceIgnoresPreviousIncarnation(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	g.Objs = append(g.Objs[:0], state.Object{ID: 1, Incarnation: 3}, state.Object{ID: 2, ExiledWith: 1, ExiledWithInc: 4})
	src, card := &g.Objs[0], &g.Objs[1]
	if !exiledBySource(g, card, 1) {
		t.Fatal("same incarnation must match")
	}
	src.Incarnation += 2
	if exiledBySource(g, card, 1) {
		t.Fatal("new incarnation must not match")
	}
}
