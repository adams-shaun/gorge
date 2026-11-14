package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBolster: bolster reads the CURRENT toughness (gorge
// putCounterBolster, h.Toughness): a tie for the least is asked
// (ChGenBolster over the tied creatures), a single least is taken unasked,
// and an until-end-of-turn pump moves the election.
func TestBolster(t *testing.T) {
	sc, c := actProliferateScen(t)
	b1, b2, big := sc.perm(c.bear, 0), sc.perm(c.bear, 0), sc.perm(c.big, 0)
	sc.perm(c.bear, 1) // the opponent's creature is never a candidate
	sc.actProliferateCast(t, c.bolster)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenBolster || d.N != 2 {
		t.Fatalf("decision kind %d sub %d with %d options, want the bolster tie ask over 2 bears", d.Kind, d.Sub, d.N)
	}
	sc.do(t, AkChoose, TCode(b2), scAny)
	if n := sc.s.perms[sc.slot(b2)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("the chosen bear has %d +1/+1 counters, want 2", n)
	}
	// b1 2/2 pumped to 2/4, big 3/3, b2 4/4: big is the single least
	sc.s.perms[sc.slot(b1)].eotT = 2
	sc.actProliferateCast(t, c.bolster)
	if d := sc.s.Step(); d.Kind == DkChoose {
		t.Fatalf("a single least-toughness creature was asked (sub %d)", d.Sub)
	}
	if n := sc.s.perms[sc.slot(big)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("the 3/3 has %d +1/+1 counters, want 2 (b1 is pumped to 2/4)", n)
	}
	if n := sc.s.perms[sc.slot(b1)].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("the pumped bear has %d counters, want 0", n)
	}
}
