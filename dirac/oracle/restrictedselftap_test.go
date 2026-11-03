package oracle

import "testing"

// An activated ability whose cost taps its own source offers only what the
// source's other mana can pay (CR 602.2, 605.3b): Isolinear Processor's
// {2},{T} draw is not funded by a land and the Processor's own restricted
// {C}, because the {T} cost leaves the Processor unable to tap for mana.
func TestRestrictedSourceTapCostExcludesItselfLockstep(t *testing.T) {
	t.Parallel()
	synthPairsReachingRun(t, "std-trk", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, []int{27, 42, 70}, nil, onBattlefield("Isolinear Processor"))
}
