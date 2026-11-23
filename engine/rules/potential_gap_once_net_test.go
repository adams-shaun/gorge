package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// gapOnceNetSupply bounds a census-missed once-only fee-bearing gap by its
// production minus fee, less the covered best only when the gap's own {T}
// makes it exclusive with the covered ability (CR 106.1, 605.3b, 601.2h).
func TestGapOnceNetSupplyGuards(t *testing.T) {
	t.Parallel()
	rr := &effects.ManaParams{Counts: [6]int32{state.MR: 2}}
	rr.AmountTrim, rr.AmountLit, rr.AmountIsLit = "1", 1, true
	dyn := &effects.ManaParams{Counts: [6]int32{state.MR: 2}}
	dyn.AmountTrim = "X"
	var none state.Mana
	var extra state.Mana
	extra[state.MR] = 1
	for _, tc := range []struct {
		name       string
		mp         *effects.ManaParams
		extra      state.Mana
		fee, best  int32
		once, taps bool
		wantN      int32
		wantOK     bool
	}{
		{"tap once fee1 best1 (Fire-Lit Thicket)", rr, none, 1, 1, true, true, 0, true},
		{"tap once fee1 best0", rr, none, 1, 0, true, true, 1, true},
		{"net not above best floors at zero", rr, none, 2, 3, true, true, 0, true},
		{"non-tap once-only gap beside a covered tap ability", rr, none, 1, 1, true, false, 0, false},
		{"repeatable gap", rr, none, 1, 1, false, true, 0, false},
		{"no mana fee", rr, none, 0, 1, true, true, 0, false},
		{"dynamic amount", dyn, none, 1, 1, true, true, 0, false},
		{"tap trigger adds mana", rr, extra, 1, 1, true, true, 0, false},
	} {
		n, ok := gapOnceNetSupply(tc.mp, tc.extra, tc.fee, tc.best, tc.once, tc.taps)
		if n != tc.wantN || ok != tc.wantOK {
			t.Errorf("%s: gapOnceNetSupply = (%d, %v), want (%d, %v)", tc.name, n, ok, tc.wantN, tc.wantOK)
		}
	}
}
