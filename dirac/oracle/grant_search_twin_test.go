package oracle

import "testing"

// TestWorldTreeSearchReprosLockstep replays tail-08 pair 78 (the recording):
// at turn 34's upkeep gorge offered The World Tree's search activation and
// the kernel dropped it. The board is five Plains and five Islands whose
// 6-land static granted "{T}: Add one mana of any color" and a Dwarven Hold
// with two storage counters, all untapped; the cost is {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}.
// gorge prices the play with its one prefix search -- the planner census
// never covers a granted mana ability, so the verdict is census_incomplete
// and the search's subsume run finds the paying script. The kernel's grant
// search walked the granted lands as distinct subtrees and spent the shared
// node budget on their permutations before the paying colour assignment, so
// its subsume ran out one branch short; it now twin-cuts the gap candidates
// like gorge's scriptTwin (mana_grantedoffers.go gapTwinSkip) and prices the
// play the same way.
func TestWorldTreeSearchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_prices_activated_cost")
}
