package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPutCountersFrom: a dies trigger's OpPutCountersFrom copies every
// counter kind the dead source had, from its LKI snapshot (gorge
// putCounterEachFromSource rung 2), CounterNum$ times; the recipient's
// live counters are added to, and a +1/+1 counter fires the counter-added
// trigger once as OpPutCounter would.
func TestPutCountersFrom(t *testing.T) {
	sc, c := ctrClearScen(t)
	pupil, bear := sc.perm(c.pupil, 0), sc.perm(c.bear, 0)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrP1P1), 2)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrStun), 1)
	sc.s.perms[sc.slot(bear)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.ctrClearKill(t, c, pupil, bear)
	if p := &sc.s.perms[sc.slot(bear)]; p.ctrLow(int(kop.CtrP1P1)) != 3 || p.ctrLow(int(kop.CtrStun)) != 1 {
		t.Fatalf("bear has %d +1/+1 and %d stun, want 3 and 1 (the pupil's 2 and 1 copied from its snapshot)", p.ctrLow(int(kop.CtrP1P1)), p.ctrLow(int(kop.CtrStun)))
	}
	if sc.s.StallReason() != 0 {
		t.Fatalf("stalled %d with no counter-added trigger in the table", sc.s.StallReason())
	}
}

// TestPutCountersFromOrderStall: with a counter-added trigger in the
// table a one-kind source copies and fires the trigger once. (Two kinds that
// the trigger hears stall: TestPutCountersFromHeardOrder.)
func TestPutCountersFromOrderStall(t *testing.T) {
	sc, c := ctrClearScen(t)
	sc.perm(c.watcher, 0)
	pupil, bear := sc.perm(c.pupil, 0), sc.perm(c.bear, 0)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	life := sc.s.pl[0].life
	sc.ctrClearKill(t, c, pupil, bear)
	if p := &sc.s.perms[sc.slot(bear)]; p.ctrLow(int(kop.CtrP1P1)) != 1 || sc.s.pl[0].life != life+1 {
		t.Fatalf("one kind: bear +1/+1 %d (want 1), life %d -> %d (want +1 from the counter-added trigger)", p.ctrLow(int(kop.CtrP1P1)), life, sc.s.pl[0].life)
	}
}

// TestPutCountersFromLive: the source's live counters win over the
// snapshot (gorge rung 1): a source still on the battlefield is read as it
// is now.
func TestPutCountersFromLive(t *testing.T) {
	sc, c := ctrClearScen(t)
	pupil, bear := sc.perm(c.pupil, 0), sc.perm(c.bear, 0)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrP1P1), 2)
	s := sc.s
	// run the op directly with the pupil as the resolving entry's trigger
	// object and a stale snapshot of one counter
	s.res.e.trigObj = pupil
	s.res.e.li = s.snapLKI(pupil)
	s.perms[sc.slot(pupil)].addCtrN(s, int(kop.CtrP1P1), 1)
	s.res.e.tgt[0] = TCode(bear)
	in := kop.Instr{Op: kop.OpPutCountersFrom, Obj: kop.RefTarget0, A: kop.Const(2), C: int16(kop.RefTriggerObj)}
	s.putCountersFrom(sc.k, &in)
	if n := s.perms[sc.slot(bear)].ctrLow(int(kop.CtrP1P1)); n != 6 {
		t.Fatalf("bear has %d +1/+1, want 6 (the live 3, times 2)", n)
	}
}
