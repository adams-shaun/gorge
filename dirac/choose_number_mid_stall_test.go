package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChooseNumberMidMaxPastSentinelStalls: a present Max$ the chosen byte
// cannot keep (255 up to gorge's 1000-value ceiling) is a declared capacity
// stall, never a truncated 0..254 ask; the largest kept bound still asks.
func TestChooseNumberMidMaxPastSentinelStalls(t *testing.T) {
	cases := []struct {
		name  string
		max   int
		stall bool
	}{
		{"largest kept bound asks", 254, false},
		{"sentinel value stalls", 255, true},
		{"past the sentinel stalls", 300, true},
		{"gorge's ceiling stalls", 1000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScen(t, Config{NoElide: true})
			sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
			in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtChooseNumberMid)),
				Obj: kop.RefController, B: kop.Const(tc.max), C: 0x100}
			ok, done := sc.s.execExt(sc.k, &in)
			if !done {
				t.Fatal("ext dispatch did not claim ExtChooseNumberMid")
			}
			if tc.stall {
				if !ok {
					t.Fatal("the capacity-stalled choice suspended for an ask")
				}
				if got := sc.s.StallReason(); got != StallChooseNumber {
					t.Fatalf("stall = %q, want %q", StallName(got), StallName(StallChooseNumber))
				}
				if sc.s.res.ask == raChooseNumber {
					t.Fatal("a truncated number ask was posed")
				}
				return
			}
			if ok {
				t.Fatal("the 0..254 ask did not suspend")
			}
			if sc.s.res.ask != raChooseNumber {
				t.Fatalf("ask %d, want raChooseNumber", sc.s.res.ask)
			}
			if sc.s.Stalled() {
				t.Fatalf("the largest kept bound stalled: %q", StallName(sc.s.StallReason()))
			}
		})
	}
}
