package oracle

import "testing"

// CR 601.2f, 602.2b: an activation's total cost is determined before any of it
// is paid (CR 601.2h), so an own ReduceCost$ counting the source's counters
// reads them even though the source is sacrificed as part of the cost. In
// tail-66 pair 7 (evl-11190841) Exploding Barrel's {8}, {T}, Sacrifice ability
// was offered at its reduced price; the kernel priced the charge only after
// payAbilityCosts had sacrificed the Barrel (no counters, the full {8}), and
// its autopay found no plan. tail-14 pair 10 (evl-11190819) is the same shape
// on Quest for the Necropolis' quest counters. The charge is now composed
// before the costs are paid whenever a table holds an own ReduceCost$.
func TestOwnReduceCountersPricedBeforeSacrificeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "own_reduce_counters_priced_before_sacrifice")
}

func TestOwnReduceQuestCountersPricedBeforeSacrificeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "own_reduce_quest_counters_priced_before_sacrifice")
}
