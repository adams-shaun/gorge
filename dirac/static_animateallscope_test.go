package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOverrideAll: OpOverrideAll (gorge effAnimateAll) gives every
// battlefield permanent matching the filter, and no other, an override
// record of the given duration.
func TestOverrideAll(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}})
	if len(sc.k.t.Overrides) == 0 {
		t.Skip("no override in the BLB table")
	}
	fc := id["Valley Floodcaller"]
	a, b, c := sc.perm(fc, 0), sc.perm(fc, 0), sc.perm(fc, 1)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	f := firstInstr(t, sc, fc, kop.OpUntapAll).F // Otters you control
	in := kop.Instr{Op: kop.OpOverrideAll, F: f, A: kop.Const(kop.DurEOT), C: 0}
	sc.s.exec(sc.k, &in)
	recs := func(o Obj) (n int, eot bool) {
		sc.s.eachOvRec(sc.slot(o), func(r ovRec) {
			n++
			eot = r.fl&ovEOT != 0
		})
		return
	}
	for _, o := range []Obj{a, b} {
		if n, eot := recs(o); n != 1 || !eot {
			t.Fatalf("matching Otter: %d records (eot %v), want 1 until end of turn", n, eot)
		}
	}
	if n, _ := recs(c); n != 0 {
		t.Fatalf("the opponent's Otter got %d records, want 0", n)
	}
}
