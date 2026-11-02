package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPutCounterNegativeAmount: CounterNum$ is an effect amount, so a
// negative result is zero (CR 107.1b). Gorge clamps the amount at the effect
// entry (engine/effects/counters_put.go:37-40), before its recipient sweep
// and before any replacement, so a negative CounterNum$ places nothing and
// removes nothing. The kernel used to pass the raw value through
// replCounters to addCtrN, which treats it as a delta and lets setCtrN clamp
// the resulting count: one existing +1/+1 counter and an amount of -2 left 0
// instead of 1. replCounters is the shared amount helper of every put-counter
// path, so the clamp lives there (single-object OpPutCounter,
// putCounterRemembered and putCounterProg).
func TestPutCounterNegativeAmount(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Zimone, Paradox Sculptor", Count: 3},
	})
	b := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	a := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	c := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	sc.s.res.e.trigObj = b

	// A = -2 on a permanent holding one +1/+1 counter places nothing: the
	// count stays 1, as gorge's 1 + 0 does.
	p := &sc.s.perms[sc.slot(b)]
	p.setCtrN(sc.s, int(kop.CtrP1P1), 1)
	put := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefTriggerObj, A: kop.Const(-2), C: int16(kop.CtrP1P1)}
	sc.s.exec(sc.k, &put)
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("single-object OpPutCounter A=-2 on 1 counter = %d, want 1", n)
	}

	// The remembered arm (nrem > 1) reads CounterNum$ once and applies it to
	// each remembered permanent; the same clamp leaves each at 1.
	pa := &sc.s.perms[sc.slot(a)]
	pc := &sc.s.perms[sc.slot(c)]
	pa.setCtrN(sc.s, int(kop.CtrP1P1), 1)
	pc.setCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.g.rem[0], sc.s.g.rem[1] = a, c
	sc.s.g.nrem = 2
	rem := kop.Instr{Op: kop.OpPutCounter, Obj: kop.RefRemembered, A: kop.Const(-2), C: int16(kop.CtrP1P1)}
	sc.s.exec(sc.k, &rem)
	if na, nc := pa.ctrLow(int(kop.CtrP1P1)), pc.ctrLow(int(kop.CtrP1P1)); na != 1 || nc != 1 {
		t.Fatalf("remembered OpPutCounter A=-2 on 1 counter each = %d, %d, want 1 and 1", na, nc)
	}
	sc.s.g.nrem = 0

	// A = -2 on a permanent holding no counters stays 0: a count is never
	// negative, and no counter is removed from an empty permanent.
	d := sc.perm(id["Zimone, Paradox Sculptor"], 0)
	pd := &sc.s.perms[sc.slot(d)]
	sc.s.res.e.trigObj = d
	sc.s.exec(sc.k, &put)
	if n := pd.ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("single-object OpPutCounter A=-2 on 0 counters = %d, want 0", n)
	}
}
