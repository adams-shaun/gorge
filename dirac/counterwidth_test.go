package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterWidth: every counter kind is 16-bit, so OpPutCounter and
// OpDoubleCounters (Zimone's "each kind") keep growing past the old 8-bit
// 255 clamp, and last known information reports the full count. Zimone's
// {G}{U},{T} doubling of 141 +1/+1 counters is pair 1179's divergence.
func TestCounterWidth(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Zimone, Paradox Sculptor", Count: 1},
		{Name: "Island", Count: 1},
		{Name: "Forest", Count: 1},
	})
	z := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	sl := sc.slot(z)
	p := &sc.s.perms[sl]
	sc.s.res.e.trigObj = z

	// OpPutCounter across the old 255 boundary, for +1/+1 and another kind.
	p.setCtrN(sc.s, int(kop.CtrP1P1), 250)
	p.setCtrN(sc.s, int(kop.CtrSoul), 250)
	put := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefTriggerObj, A: kop.Const(1), C: int16(kop.CtrP1P1)}
	sc.s.exec(sc.k, &put)
	put.C = int16(kop.CtrSoul)
	sc.s.exec(sc.k, &put)
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 251 {
		t.Fatalf("+1/+1 count after OpPutCounter at 250 = %d, want 251", n)
	}
	if n := p.ctrN(sc.s, int(kop.CtrSoul)); n != 251 {
		t.Fatalf("soul counter count after OpPutCounter at 250 = %d, want 251", n)
	}

	// OpDoubleCounters: 141 -> 282, then 282 -> 564 (no re-clamp).
	p.setCtrN(sc.s, int(kop.CtrP1P1), 141)
	dbl := kop.Instr{Op: kop.OpDoubleCounters, Obj: kop.RefTriggerObj, C: 255}
	sc.s.exec(sc.k, &dbl)
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 282 {
		t.Fatalf("+1/+1 count after first doubling = %d, want 282", n)
	}
	sc.s.exec(sc.k, &dbl)
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 564 {
		t.Fatalf("+1/+1 count after second doubling = %d, want 564", n)
	}

	// Last known information keeps the full counts, not a wrapped nibble.
	p.setCtrN(sc.s, int(kop.CtrP1P1), 280)
	p.setCtrN(sc.s, int(kop.CtrSoul), 280)
	li := sc.s.snapLKI(z)
	if n := sc.s.lkiCounter(li, kop.CtrP1P1); n != 280 {
		t.Fatalf("last known +1/+1 count = %d, want 280", n)
	}
	if n := sc.s.lkiCounter(li, kop.CtrSoul); n != 280 {
		t.Fatalf("last known soul counter count = %d, want 280", n)
	}
}
