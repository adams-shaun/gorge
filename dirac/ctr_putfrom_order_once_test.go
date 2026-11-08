package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPutCountersFromOnceListenerOrder: a source with a +1/+1 and a flying
// counter is copied under one any-kind CounterAdded listener with
// ActivationLimit$ 1. The listener hears both kinds, but can fire only for
// the first put, and the same ability fires whichever kind came first, so
// the copy's kind order is unobservable and does not stall (CR 603.3b).
// Without the once-per-turn limit the two triggers' stack order is the
// insertion order and the copy still stalls (TestPutCountersFromHeardOrder).
func TestPutCountersFromOnceListenerOrder(t *testing.T) {
	sc, c := ctrClearScenHears(t, kop.TrigNAnyCounter)
	watcher := &sc.k.t.Cards[c.watcher].Abilities[0]
	watcher.Trig.Flags |= kop.TfOncePerTurn
	sc.perm(c.watcher, 0)
	if !sc.s.ctrAddedHeard(sc.k, kop.CtrP1P1) || !sc.s.ctrAddedHeard(sc.k, kop.CtrFlying) {
		t.Fatal("the listener must hear both kinds")
	}
	if watcher.Trig.N != kop.TrigNAnyCounter || watcher.Trig.Flags&kop.TfOncePerTurn == 0 || watcher.Gate.Ctr != 0 {
		t.Fatal("the fixture listener is not an ungated any-kind once-per-turn trigger")
	}
	pupil, bear := sc.perm(c.pupil, 0), sc.perm(c.bear, 0)
	p := &sc.s.perms[sc.slot(pupil)]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 1)
	p.addCtrN(sc.s, int(kop.CtrFlying), 1)
	life := sc.s.pl[0].life
	sc.ctrClearKill(t, c, pupil, bear)
	if r := sc.s.StallReason(); r != 0 {
		t.Fatalf("stalled %d: one once-per-turn listener makes the two kinds' order unobservable", r)
	}
	if q := &sc.s.perms[sc.slot(bear)]; q.ctrLow(int(kop.CtrP1P1)) != 1 || q.ctrLow(int(kop.CtrFlying)) != 1 {
		t.Fatalf("bear +1/+1 %d flying %d, want 1 and 1", q.ctrLow(int(kop.CtrP1P1)), q.ctrLow(int(kop.CtrFlying)))
	}
	if got := sc.s.pl[0].life; got != life+1 {
		t.Fatalf("life %d -> %d, want one trigger (+1)", life, got)
	}
}
