package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestConvokeFrameNoXRefusesSpentGeneric: a {1}{G} printed-Convoke cast with
// two non-green creatures (no {X}). gorge leaves Max == #options (2 options,
// 2 slots), yet after one generic pick it refuses the second (convokeAbsorbs,
// xOpen=false), as the kernel's re-ask drops it. An uncapped Max is not the
// open-X signal; only the cast's {X} is.
func TestConvokeFrameNoXRefusesSpentGeneric(t *testing.T) {
	t.Parallel()

	const spell, a, b = state.ObjID(10), state.ObjID(41), state.ObjID(42)
	var pips decision.ManaAmount
	pips[state.ManaIndex('G')] = 1
	d := &decision.Decision{Kind: decision.KChoose, Source: spell, Min: 0, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "convoke_generic", Obj: a, Group: "payment:41"},
			{Index: 1, Kind: "convoke_generic", Obj: b, Group: "payment:42"},
		}}
	g := &GorgeSide{castCost: map[state.ObjID]decision.PaymentCost{spell: {Generic: 1, Mana: pips}},
		castX: map[state.ObjID]bool{spell: false}}
	fr := g.convokeFrameFor(d)
	if fr == nil {
		t.Fatal("no convoke frame for a cached cast")
	}
	if d.Max != len(d.Options) {
		t.Fatalf("precondition: Max %d must equal #options %d", d.Max, len(d.Options))
	}
	if !fr.admits(d, nil, 0) {
		t.Fatal("the first generic pick must be admitted")
	}
	if fr.admits(d, []int{0}, 1) {
		t.Fatal("a second generic pick on {1}{G} was admitted; gorge refuses it")
	}

	// The same menu on an {X} cast keeps the tentative pre-X generic.
	g.castX[spell] = true
	if fx := g.convokeFrameFor(d); !fx.admits(d, []int{0}, 1) {
		t.Fatal("an {X} cast must admit a generic pick past the printed generic")
	}
}
