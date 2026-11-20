package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestProliferateFilteredListenerOrder: a counter-added trigger whose event
// filter excludes the recipient ("on a creature you don't control") does not
// hear the proliferate's per-kind puts, so a recipient with several kinds
// does not stall on their order (CR 603.2, CR 603.3b). The same listener
// without a filter still stalls (TestProliferateTwoKindsUnheardOrder).
func TestProliferateFilteredListenerOrder(t *testing.T) {
	sc, c := actProliferateHeardScen(t, kop.TrigNAnyCounter)
	// The watcher hears counters only on a creature its controller does not
	// control; the bear is the watcher's own.
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
	trig := &sc.k.t.Cards[c.watcher].Abilities[0].Trig
	trig.Filter = kop.FilterID(len(sc.k.t.Filters) - 1)
	sc.perm(c.watcher, 0)
	bear := sc.perm(c.bear, 0)
	sl := sc.slot(bear)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrLifelink), 1)
	if sc.s.ctrAddedHeard(sc.k, kop.CtrP1P1, sl) || sc.s.ctrAddedHeard(sc.k, kop.CtrLifelink, sl) {
		t.Fatal("precondition: the filtered watcher must not hear the bear")
	}
	if !sc.s.ctrAddedHeard(sc.k, kop.CtrP1P1, NoSlot) {
		t.Fatal("precondition: an unknown recipient is a superset and must hear the put")
	}
	life := sc.s.pl[0].life
	sc.actProliferateCast(t, c.prolif)
	if d := sc.s.Step(); d.Kind != DkChoose || d.Sub != ChGenProlif || d.N != 2 {
		t.Fatalf("decision kind %d sub %d with %d options, want the proliferate pick (the bear, Done)", d.Kind, d.Sub, d.N)
	}
	sc.do(t, AkChoose, TCode(bear), scAny)
	if r := sc.s.StallReason(); r != 0 {
		t.Fatalf("stalled %d: the watcher's filter excludes the recipient", r)
	}
	p := &sc.s.perms[sl]
	if p.ctrN(sc.s, int(kop.CtrP1P1)) != 2 || p.ctrN(sc.s, int(kop.CtrLifelink)) != 2 {
		t.Fatalf("bear +1/+1 %d lifelink %d, want 2 and 2", p.ctrN(sc.s, int(kop.CtrP1P1)), p.ctrN(sc.s, int(kop.CtrLifelink)))
	}
	if got := sc.s.pl[0].life; got != life {
		t.Fatalf("life %d -> %d: the filtered watcher must not fire", life, got)
	}
}
