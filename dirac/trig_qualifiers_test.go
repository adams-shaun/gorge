package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTrigCmpOK: a bare literal is the card's default operator (>= for the
// attackers gate, == for the N-th spell gate); a kop.DmgGate packs the other
// operators; DebugTrigQualsFlip inverts every answer.
func TestTrigCmpOK(t *testing.T) {
	for _, c := range []struct {
		have     int
		n        int16
		zeroIsGE bool
		want     bool
	}{
		{2, 2, true, true}, {3, 2, true, true}, {1, 2, true, false},
		{2, 2, false, true}, {3, 2, false, false}, {1, 2, false, false},
		{1, kop.DmgGate(kop.DmgEQ, 1), true, true}, {2, kop.DmgGate(kop.DmgEQ, 1), true, false},
		{2, kop.DmgGate(kop.DmgGT, 1), false, true}, {1, kop.DmgGate(kop.DmgGT, 1), false, false},
		{1, kop.DmgGate(kop.DmgLT, 3), true, true}, {3, kop.DmgGate(kop.DmgLT, 3), true, false},
		{2, kop.DmgGate(kop.DmgNE, 2), false, false}, {3, kop.DmgGate(kop.DmgNE, 2), false, true},
	} {
		if got := trigCmpOK(c.have, c.n, c.zeroIsGE); got != c.want {
			t.Errorf("trigCmpOK(%d, %#x, %v) = %v, want %v", c.have, c.n, c.zeroIsGE, got, c.want)
		}
		DebugTrigQualsFlip = true
		if got := trigCmpOK(c.have, c.n, c.zeroIsGE); got == c.want {
			t.Errorf("flipped trigCmpOK(%d, %#x, %v) = %v", c.have, c.n, c.zeroIsGE, got)
		}
		DebugTrigQualsFlip = false
	}
}
