package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCastReturnedCount pins the `CastSA>Returned$CardManaCost` count row
// (Scarlet Spider, Ben Reilly's X): the mana value of the cards the cast's
// Return cost returned.
func TestCastReturnedCount(t *testing.T) {
	h := newHost(t, 1)
	card := mkCard(t, "Name:Ret Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear := h.g.AddObject(card, 0)
	const expr = "CastSA>Returned$CardManaCost"
	for _, tc := range []struct {
		name string
		ids  []state.ObjID
		want int32
	}{
		{"returned card reads its mana value", []state.ObjID{bear.ID}, 2},
		{"no return is a resolved zero", nil, 0},
		{"a ceased token reads zero", []state.ObjID{9999}, 0},
	} {
		c := &Ctx{Source: 99, Controller: 0}
		c.Paid.Returned = tc.ids
		got, ok := EvalCountOK(h, c, expr)
		if !ok || got != tc.want {
			t.Errorf("%s: %s = (%d, %v), want (%d, true)", tc.name, expr, got, ok, tc.want)
		}
	}
}
