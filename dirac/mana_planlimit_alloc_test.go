package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestLimitPlanAllocatesNothing: the node-limited replay of the mana plan
// (limitPlan) is on Apply's and StepInto's path, where a transition allocates
// nothing. Its search state is a stack variable only while it stays under the
// compiler's 128 KiB limit for one.
func TestLimitPlanAllocatesNothing(t *testing.T) {
	sc, id := ktreScen(t)
	set := cardsettest.Set(t, "sacmana")
	treasure := CardID(set.PoolID(set.NumCards()))
	for i := 0; i < 6; i++ {
		sc.perm(id("Forest"), 0)
	}
	for i := 0; i < 12; i++ {
		sc.perm(treasure, 0)
	}
	// plan {14} from eighteen sources without tapping them
	c := cost(14)
	var pk planner
	pk.dry = true
	sc.s.tapForPK(sc.k, 0, &c, 0, NoObj, false, false, &pk)
	if !pk.found || pk.nu < planLimitUnits {
		t.Fatalf("found=%v units=%d; want a plan over at least %d sources", pk.found, pk.nu, planLimitUnits)
	}
	if pk.bfl || pk.anyColor || pk.pairs || pk.rdynOn {
		t.Fatalf("the plan %+v is one limitPlan declines", pk)
	}
	for ti := 0; ti < pk.nt; ti++ {
		if ty := &pk.types[ti]; ty.sacOnly || ty.first != 0 || ty.pain != 0 || ty.dmask != ty.mask {
			t.Fatalf("type %d %+v is one limitPlan declines", ti, *ty)
		}
	}
	var ls limitSearch
	ls.pk = pk
	ls.census(sc.k, sc.s)
	ls.order()
	ls.walk(0)
	if ls.nodes == 0 || !ls.found {
		t.Fatalf("the replay visited %d nodes and found=%v; want a search that finds a plan", ls.nodes, ls.found)
	}
	if allocs := testing.AllocsPerRun(20, func() {
		p := pk
		sc.s.limitPlan(sc.k, &p)
	}); allocs != 0 {
		t.Errorf("limitPlan allocates %v times per run", allocs)
	}
}
