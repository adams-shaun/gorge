package oracle

import "testing"

// A reflected Amount$ source (Incubation Druid, adapted) beside tapped Forests
// is the only green on p0's board (tail-55-c000 pair 7). gorge's potential walk
// prices the source unbounded in {C} and in each colour the lands could produce
// (CR 106.7), so it lists the plays that need that green and runs a prefix
// search for each. Both recordings are decisions where dirac's candidate walk
// dropped such a play gorge offered:
//   - whip_silk_cast_offer: Whip Silk's {G} cast (decision 474);
//   - reflamt_filter_board_bound_play: Silvos, Rogue Elemental's {3}{G}{G}{G}
//     cast (decision 621) once Cascade Bluffs' filter is live beside the Druid.
//     caBoard refuses a filter board, so the candidate gate's bound arm
//     (reflAmtBoundPlay) must ask caAny, which the filter search's
//     otherComboAny step covers, not ca.
func TestMBCWhipSilkReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"whip_silk_cast_offer", "reflamt_filter_board_bound_play"} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
