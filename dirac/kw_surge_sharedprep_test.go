package dirac

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSurgePlannerPricesKeepsSharedPrep: the surge probe's census leaves the
// sacrifice-only sources out (surgePlannerPrices), but that exclusion is the
// probing game's alone. The table's prep is shared by every game playing it,
// so a second game on the same table must keep pricing its Eldrazi Spawn as a
// sacrifice-only source while the first is inside the probe.
func TestSurgePlannerPricesKeepsSharedPrep(t *testing.T) {
	sc, _, objs := spawnScen(t, Config{NoElide: true, GorgeLowering: true}, "c_0_1_eldrazi_spawn_sac", "Swamp", "Forest")
	k := sc.s.k()
	spawn := sc.slot(objs[0])
	if !k.sacOnlyMana || !sc.s.canSacOnlyAt(k, spawn) {
		t.Fatalf("precondition: the Spawn is a sacrifice-only source (sacOnlyMana %v)", k.sacOnlyMana)
	}
	// {1}{B}{G}: the two lands plus the Spawn pay it; the lands alone do not.
	var cost kop.ManaCost
	cost.Generic, cost.Pips[kop.B], cost.Pips[kop.G] = 1, 1, 1
	var full avail
	sc.s.availFor(k, 0, &full)
	if !full.affords(&cost, pipMaskOf(&cost), 0) {
		t.Fatal("precondition: the offer census (Spawn included) pays {1}{B}{G}")
	}
	if sc.s.surgePlannerPrices(k, 0, 0, NoObj, &cost) {
		t.Fatal("the surge probe's census prices {1}{B}{G}: it must leave the sacrifice-only Spawn out")
	}
	if !k.sacOnlyMana || !sc.s.canSacOnlyAt(k, spawn) {
		t.Fatal("after the probe the Spawn is no longer a sacrifice-only source")
	}

	other := new(State)
	other.CopyFrom(sc.s)
	if other.k() != k {
		t.Fatal("precondition: the two games share one prep")
	}
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			sc.s.surgePlannerPrices(k, 0, 0, NoObj, &cost)
		}
	}()
	lost := 0
	for i := 0; i < 200000; i++ {
		if !other.canSacOnlyAt(k, spawn) {
			lost++
		}
	}
	stop.Store(true)
	wg.Wait()
	if lost != 0 {
		t.Fatalf("the other game lost its sacrifice-only source %d times while the first ran the surge probe", lost)
	}
}
