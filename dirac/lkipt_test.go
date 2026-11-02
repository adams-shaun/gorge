package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLKIPowerPast127: last known power/toughness holds more than 127
// (CR 608.2h, pair 5419's projection family: a Burglar Rat with 130 +1/+1
// counters is 131/131 when it leaves). Before, lki stored int8 and the
// snapshot wrapped to -125.
func TestLKIPowerPast127(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Swamp", Count: 8}})
	rat := sc.perm(id["Burglar Rat"], 0)
	sc.s.perms[sc.slot(rat)].setCtrN(sc.s, int(kop.CtrP1P1), 130)
	li := sc.s.snapLKI(rat)
	l := sc.s.lk(li)
	if li == 0 || int(l.power) != 131 || int(l.tough) != 131 {
		t.Fatalf("snapLKI %d/%d (idx=%d), want 131/131", l.power, l.tough, li)
	}
	sc.s.res.e = stackEntry{kind: seSpell, li: li, tgt: noTargets}
	if got := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetPower}); got != 131 {
		t.Fatalf("VTargetPower of the departed Rat = %d, want 131", got)
	}
	if got := sc.s.value(sc.k, kop.ValueRef{K: kop.VTargetToughness}); got != 131 {
		t.Fatalf("VTargetToughness of the departed Rat = %d, want 131", got)
	}
}

// TestTriggerPowerLiveThenLKI: a trigger's "X = its power" (kgen's
// TriggeredCard$CardPower, kop.ExPowerOf over RefTriggerObj) reads the
// triggering creature's live power at resolution while it is still the same
// object on the battlefield, and its last known power once it has left
// (CR 608.2h, 400.7; gorge evaluates CounterNum$ at resolution: The Roaring
// Toeclaws, Experiment Twelve). Before, the kernel always read the snapshot
// taken as the trigger was queued. Another card's ETB trigger reads that
// card's printed power instead (GORGE-FINDINGS.md D-etbpower: gorge's look-back object
// is the pre-move card; Railway Brawler + Stingerback Terror).
func TestTriggerPowerLiveThenLKI(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Burglar Rat", Count: 1}, {Name: "Gleaming Barrier", Count: 1}, {Name: "Swamp", Count: 8}})
	rat := sc.perm(id["Burglar Rat"], 0)
	bar := sc.perm(id["Gleaming Barrier"], 0)
	// the resolving entry is an LKI holder, so its snapshot stays live; a
	// non-ETB trigger (the Barrier's) naming the Rat as its object
	sc.s.res.e = stackEntry{kind: seTrigger, obj: bar, srcVer: sc.s.objs[bar].ver, card: sc.s.objs[bar].card, trigObj: rat}
	e := &sc.s.res.e
	e.li = sc.s.snapLKI(rat)
	if e.li == 0 {
		t.Fatal("no snapshot")
	}
	sc.s.perms[sc.slot(rat)].setCtrN(sc.s, int(kop.CtrP1P1), 2)
	if p, _ := sc.s.refPT(sc.k, kop.RefTriggerObj, sc.s.ctxEntry(e)); p != 3 {
		t.Fatalf("power on the battlefield: %d, want the live 3", p)
	}
	// an ETB trigger of another card or its own reads it live, and what it was as it
	// left once it has (CR 608.2h, 603.10)
	etb := stackEntry{kind: seTrigger, obj: rat + 1, srcVer: sc.s.objs[rat].ver, card: sc.s.objs[rat].card, trigObj: rat, li: e.li}
	if p, _ := sc.s.refPT(sc.k, kop.RefTriggerObj, sc.s.ctxEntry(&etb)); p != 3 {
		t.Fatalf("an ETB trigger's power: %d, want the live 3", p)
	}
	sc.s.push(etb)
	sc.s.leave(sc.k, sc.slot(rat), kop.ZoneGraveyard, false)
	if p, _ := sc.s.refPT(sc.k, kop.RefTriggerObj, sc.s.ctxEntry(e)); p != 1 {
		t.Fatalf("power after it left: %d, want the last known 1", p)
	}
	if p, _ := sc.s.refPT(sc.k, kop.RefTriggerObj, sc.s.ctxEntry(&sc.s.stack[sc.s.sp-1])); p != 3 {
		t.Fatalf("an ETB trigger's power after it left: %d, want the 3 it left with", p)
	}
}
