package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A distinct pair (Interplanar Beacon's "{1}, {T}: Add two mana of different
// colors", CR 106.1) is two any-colour units of the availability model that
// make at most one mana of any one colour: avail.nDist takes one unit per pair
// out of a single-colour subset's supply, so {W}{W} is never over-offered.
func TestAvailDistinctPairNeverPaysTwoOfOneColour(t *testing.T) {
	pips := func(w, u int) *kop.ManaCost {
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U] = uint8(w), uint8(u)
		return &c
	}
	pair := func(n int, coupled bool) avail {
		var a avail
		a.addUnits(uint8(kop.ColAny), uint8(2*n))
		if coupled {
			a.nDist = uint8(n)
		}
		return a
	}
	for _, tc := range []struct {
		name         string
		pairs        int
		cost         *kop.ManaCost
		extra        int
		want, uncoup bool
	}{
		{"one pair pays {W}{U}", 1, pips(1, 1), 0, true, true},
		{"one pair never pays {W}{W}", 1, pips(2, 0), 0, false, true},
		{"one pair never pays {U}{U}", 1, pips(0, 2), 0, false, true},
		{"one pair pays {W} and a generic", 1, pips(1, 0), 1, true, true},
		{"two pairs pay {W}{W}", 2, pips(2, 0), 0, true, true},
		{"two pairs never pay {W}{W}{W}", 2, pips(3, 0), 0, false, true},
		{"two pairs pay {W}{W}{U}{U}", 2, pips(2, 2), 0, true, true},
	} {
		a := pair(tc.pairs, true)
		if got := a.affords(tc.cost, pipMaskOf(tc.cost), tc.extra); got != tc.want {
			t.Errorf("%s: coupled affords = %v, want %v", tc.name, got, tc.want)
		}
		// the precondition the coupling decides: the same units read as independent
		// any-colour mana over-offer the cost
		u := pair(tc.pairs, false)
		if got := u.affords(tc.cost, pipMaskOf(tc.cost), tc.extra); got != tc.uncoup {
			t.Errorf("%s: uncoupled affords = %v, want %v", tc.name, got, tc.uncoup)
		}
	}
}
