package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseNumberMidBounds pins the offer both the ask (interp_ask.go) and
// the unasked record (repl_choosenumber.go) use. It must follow gorge's
// chooseNumberAsk: a Min$ past the default list with no Max$ fails closed to
// 0, and a bound past the chosen byte's 255 no-choice sentinel stops at 254.
func TestChooseNumberMidBounds(t *testing.T) {
	cases := []struct {
		name string
		c    int16
		b    kop.ValueRef
		lo   int
		hi   int
	}{
		{name: "no bound offers the default list", c: 0, lo: 0, hi: 12},
		{name: "min floors the default list", c: 2, lo: 2, hi: 12},
		{name: "min past the default list fails closed", c: 13, lo: 0, hi: 0},
		{name: "max bounds the list", c: 0x100, b: kop.Const(5), lo: 0, hi: 5},
		{name: "min and max", c: 0x101, b: kop.Const(5), lo: 1, hi: 5},
		{name: "inverted bound fails closed", c: 0x106, b: kop.Const(5), lo: 0, hi: 0},
		{name: "past gorge's ceiling fails closed", c: 0x100, b: kop.Const(1001), lo: 0, hi: 0},
		{name: "offer stops at the no-choice sentinel", c: 0x100, b: kop.Const(300), lo: 0, hi: 254},
	}
	var s State
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := kop.Instr{C: tc.c, B: tc.b}
			lo, hi := s.chooseNumberMidBounds(nil, &in)
			if lo != tc.lo || hi != tc.hi {
				t.Fatalf("bounds = %d..%d, want %d..%d", lo, hi, tc.lo, tc.hi)
			}
		})
	}
}
