package oracle

import "testing"

// A Convoke announcement on a cast gorge's offer made through a play
// permission (CR 702.51a, 601.2h) is filtered by the outstanding absorption
// budget the announcement itself publishes (Decision.ConvokeBudget, the same
// numbers the engine's submit gate prices the answer against): a pick of a
// colour whose last pip an earlier pick paid is no longer offered, and the
// kernel poses the same menu from that one published copy.
func TestMayplayConvokeOptionsLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-ecl", []stdRepro{
		{"mayplay_convoke_options", false, false},
	})
}
