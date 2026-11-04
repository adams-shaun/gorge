package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ctrPutFromTwoKinds kills a pupil holding a +1/+1 and a stun counter into a
// bear, with the table's counter-added watcher listening for hears.
func ctrPutFromTwoKinds(t *testing.T, hears int16) (*scen, ctrClearCards, Obj) {
	t.Helper()
	sc, c := ctrClearScenHears(t, hears)
	sc.perm(c.watcher, 0)
	pupil, bear := sc.perm(c.pupil, 0), sc.perm(c.bear, 0)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sc.slot(pupil)].addCtrN(sc.s, int(kop.CtrStun), 1)
	sc.ctrClearKill(t, c, pupil, bear)
	return sc, c, bear
}

// TestPutCountersFromHeardOrder: gorge puts a source's kinds in its insertion
// order, which the kernel does not keep (it walks +1/+1 then the slots
// ascending). Two kinds that a counter-added trigger hears stall
// (StallCtrOrder: their triggers' stack order is the insertion order); two
// kinds with one heard (a +1/+1 trigger, a stun counter beside it) put in
// either order to the same state and one trigger, so they copy (CR 603.3b).
func TestPutCountersFromHeardOrder(t *testing.T) {
	sc, _, bear := ctrPutFromTwoKinds(t, kop.TrigNAnyCounter)
	if sc.s.StallReason() != StallCtrOrder {
		t.Fatalf("two kinds under an any-kind trigger: stall %d, want StallCtrOrder", sc.s.StallReason())
	}
	_ = bear
	sc, c, bear := ctrPutFromTwoKinds(t, int16(kop.CtrP1P1))
	if sc.s.StallReason() != 0 {
		t.Fatalf("a stun put beside +1/+1 counters under a +1/+1 trigger stalled %d", sc.s.StallReason())
	}
	_ = c
	p := &sc.s.perms[sc.slot(bear)]
	if p.ctrLow(int(kop.CtrP1P1)) != 1 || p.ctrLow(int(kop.CtrStun)) != 1 || sc.s.pl[0].life != 21 {
		t.Fatalf("bear +1/+1 %d stun %d, life %d; want 1, 1 and one trigger (21)", p.ctrLow(int(kop.CtrP1P1)), p.ctrLow(int(kop.CtrStun)), sc.s.pl[0].life)
	}
}
