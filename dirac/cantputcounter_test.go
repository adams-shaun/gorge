package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantPutCounterFunnel: Blossombind's S:Mode$ CantPutCounter | ValidCard$
// Card.EnchantedBy reaches the one counter-placement funnel. A counter put
// on the enchanted creature is swallowed (no counter, no replacement, no
// touch), while the same put on another creature still lands. The static's
// source is the Aura, its controller the Aura's controller, so the
// EnchantedBy filter must be read in that frame (gorge PutCounterBlocked).
func TestCantPutCounterFunnel(t *testing.T) {
	set := cardsettest.Set(t, "anykindcounters")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Skipf("%s not in the anykindcounters pool", name)
		}
		return CardID(set.PoolID(i))
	}
	lib := lookup("Plains")
	sc := newScenTableLib(t, tbl, lib, Config{NoElide: true})
	if !sc.k.gen.anyCtrBlock {
		t.Fatal("prep missed the pool's StatCantPutCounter")
	}
	host := sc.perm(lookup("Grizzly Bears"), 0)
	other := sc.perm(lookup("Grizzly Bears"), 0)
	aura := sc.perm(lookup("Blossombind"), 0)
	hostSl, otherSl, auraSl := sc.slot(host), sc.slot(other), sc.slot(aura)
	sc.s.perms[auraSl].attached = hostSl
	if !sc.s.putCounterBlocked(sc.k, hostSl, int(kop.CtrP1P1)) {
		t.Fatal("putCounterBlocked missed the enchanted creature")
	}
	if sc.s.putCounterBlocked(sc.k, otherSl, int(kop.CtrP1P1)) {
		t.Fatal("putCounterBlocked blocked an unrelated creature")
	}
	if n := sc.s.placeCounters(sc.k, hostSl, int(kop.CtrP1P1), 2, 0, ctrPutEffect); n != 0 {
		t.Fatalf("placed %d counters on the enchanted creature, want 0", n)
	}
	if n := sc.s.perms[hostSl].ctrLow(int(kop.CtrP1P1)); n != 0 {
		t.Fatalf("enchanted creature holds %d +1/+1 counters, want 0", n)
	}
	if n := sc.s.placeCounters(sc.k, otherSl, int(kop.CtrP1P1), 2, 0, ctrPutEffect); n != 2 {
		t.Fatalf("placed %d counters on the other creature, want 2", n)
	}
	if n := sc.s.perms[otherSl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("other creature holds %d +1/+1 counters, want 2", n)
	}

	// The CounterType$ gate: retype the Aura's static to +1/+1 only. A
	// loyalty counter then goes through, a +1/+1 counter still does not.
	typed := false
	auraCard := sc.s.objs[aura].card
	for i := range tbl.Cards[auraCard].Abilities {
		a := &tbl.Cards[auraCard].Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantPutCounter {
			a.Static.A = int16(kop.CtrP1P1)
			typed = true
		}
	}
	if !typed {
		t.Fatal("Blossombind has no StatCantPutCounter in the pool table")
	}
	if sc.s.putCounterBlocked(sc.k, hostSl, int(kop.CtrLoyalty)) {
		t.Fatal("CounterType$ P1P1 blocked a loyalty counter")
	}
	if n := sc.s.placeCounters(sc.k, hostSl, int(kop.CtrLoyalty), 1, 0, ctrPutEffect); n != 1 {
		t.Fatalf("placed %d loyalty counters on the enchanted creature, want 1", n)
	}
	if n := sc.s.placeCounters(sc.k, hostSl, int(kop.CtrP1P1), 1, 0, ctrPutEffect); n != 0 {
		t.Fatalf("CounterType$ P1P1 let %d +1/+1 counters through", n)
	}
}
