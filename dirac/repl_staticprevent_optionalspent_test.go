package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStaticPreventOptionalSkipEndsWithTheShield: an Optional$ shield posed
// beside two full preventions offers its skip only until it is spent. Gorge
// re-collects its candidates after each pick and offers "do not apply" only
// while an optional replacement is still applicable
// (replacement_choice.go hasOptionalReplacement), so round 2 (the two full
// preventions left, the shield picked) has no skip.
func TestStaticPreventOptionalSkipEndsWithTheShield(t *testing.T) {
	tb := newSPTable()
	h := tb.host("Test Alchemist", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou | kop.ReplSPOpp | kop.ReplSPOptional, A: 2})
	sc := tb.scen(t)
	host := sc.perm(h, 1)
	prevA, prevB := sc.perm(v.HillGiant, 0), sc.perm(v.HillGiant, 0)
	s := sc.s
	ro := &s.g.ro
	ro.on = true
	ro.n, ro.p, ro.an = 3, 0, 5
	ro.cand[0], ro.cand[1], ro.cand[2] = prevA, prevB, host
	ro.prev = 0b011
	ro.spv = 1<<2 | spOptionalAsk
	var d Decision
	s.replOrderOptions(&d)
	if d.N != 4 {
		t.Fatalf("round 1: %d options, want the two full preventions, the shield and its skip", d.N)
	}
	if !s.replOrderPick(MakeAction(AkChoose, TCode(host), 0, 2)) {
		t.Fatal("the shield picked beside two full preventions did not ask again")
	}
	if ro.an != 3 {
		t.Fatalf("amount left %d, want 3 (5 less the shield's 2)", ro.an)
	}
	d = Decision{}
	s.replOrderOptions(&d)
	if d.N != 2 {
		t.Fatalf("round 2: %d options %v, want the two full preventions alone (no skip: the optional line is spent)", d.N, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == ro.n {
			t.Fatalf("round 2 offers the skip: %v", d.Options[:d.N])
		}
	}
}
