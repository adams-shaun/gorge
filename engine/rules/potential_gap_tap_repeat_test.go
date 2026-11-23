package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// gapTapRepeatSupply bounds a census-missed repeatable tap-another gap by the
// activations the untapped candidates allow (Birchlore Rangers' two Elves,
// CR 106.1, 605.3b); a cost with anything beyond literal tap parts, a dynamic
// amount, or a tap trigger is not provable.
func TestGapTapRepeatSupplyGuards(t *testing.T) {
	t.Parallel()
	any1 := &effects.ManaParams{CountsAny: true}
	any1.AmountTrim, any1.AmountLit, any1.AmountIsLit = "1", 1, true
	dyn := &effects.ManaParams{CountsAny: true}
	dyn.AmountTrim = "X"
	var none, extra state.Mana
	extra[state.MR] = 1
	tapTwoElves := costvocab.Cost{TapPermanent: []costvocab.CostPart{{N: 2, Spec: "Elf"}}}
	tapDyn := costvocab.Cost{TapPermanent: []costvocab.CostPart{{Dyn: "X", Spec: "Elf"}}}
	for _, tc := range []struct {
		name      string
		c         costvocab.Cost
		mp        *effects.ManaParams
		extra     state.Mana
		repeats   int32
		relaxedOK bool
		wantN     int32
		wantOK    bool
	}{
		{"two Elves, one activation (Birchlore Rangers)", tapTwoElves, any1, none, 1, true, 1, true},
		{"four Elves, two activations", tapTwoElves, any1, none, 2, true, 2, true},
		{"cost cannot be paid once", tapTwoElves, any1, none, 0, true, 0, true},
		{"no relaxation bound", tapTwoElves, any1, none, 1, false, 0, false},
		{"tap the source is not a tap-another cost", costvocab.Cost{Tap: true}, any1, none, 1, true, 0, false},
		{"dynamic tap count", tapDyn, any1, none, 1, true, 0, false},
		{"dynamic amount", tapTwoElves, dyn, none, 1, true, 0, false},
		{"tap trigger adds mana", tapTwoElves, any1, extra, 1, true, 0, false},
	} {
		n, ok := gapTapRepeatSupply(tc.c, tc.mp, tc.extra, tc.repeats, tc.relaxedOK)
		if n != tc.wantN || ok != tc.wantOK {
			t.Errorf("%s: gapTapRepeatSupply = (%d, %v), want (%d, %v)", tc.name, n, ok, tc.wantN, tc.wantOK)
		}
	}
}
