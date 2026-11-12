package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseNumberMidBounds pins the offer both the ask (interp_ask.go) and
// the unasked record (repl_choosenumber.go) use. It follows gorge's
// chooseNumberAsk: a Min$ past the default list with no Max$ fails closed to
// 0, and a present Max$ the chosen byte cannot keep (above 254, gorge's own
// offer reaching 1000) is reported not-ok so the caller declares the capacity
// stall instead of asking a shorter list.
func TestChooseNumberMidBounds(t *testing.T) {
	cases := []struct {
		name string
		c    int16
		b    kop.ValueRef
		lo   int
		hi   int
		ok   bool
	}{
		{name: "no bound offers the default list", c: 0, lo: 0, hi: 12, ok: true},
		{name: "min floors the default list", c: 2, lo: 2, hi: 12, ok: true},
		{name: "min past the default list fails closed", c: 13, lo: 0, hi: 0, ok: true},
		{name: "max bounds the list", c: 0x100, b: kop.Const(5), lo: 0, hi: 5, ok: true},
		{name: "min and max", c: 0x101, b: kop.Const(5), lo: 1, hi: 5, ok: true},
		{name: "inverted bound fails closed", c: 0x106, b: kop.Const(5), lo: 0, hi: 0, ok: true},
		{name: "past gorge's ceiling fails closed", c: 0x100, b: kop.Const(1001), lo: 0, hi: 0, ok: true},
		{name: "sentinel value is capacity", c: 0x100, b: kop.Const(255), lo: 0, hi: 0, ok: false},
		{name: "past the sentinel is capacity", c: 0x100, b: kop.Const(300), lo: 0, hi: 0, ok: false},
		{name: "gorge's ceiling is capacity", c: 0x100, b: kop.Const(1000), lo: 0, hi: 0, ok: false},
		{name: "the largest kept bound asks", c: 0x100, b: kop.Const(254), lo: 0, hi: 254, ok: true},
	}
	var s State
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := kop.Instr{C: tc.c, B: tc.b}
			lo, hi, ok := s.chooseNumberMidBounds(nil, &in)
			if lo != tc.lo || hi != tc.hi || ok != tc.ok {
				t.Fatalf("bounds = %d..%d ok=%v, want %d..%d ok=%v", lo, hi, ok, tc.lo, tc.hi, tc.ok)
			}
		})
	}
}
