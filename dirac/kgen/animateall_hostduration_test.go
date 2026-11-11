package kgen

import (
	"strings"
	"testing"
)

// TestAnimateAllHostBoundDurationRefused: an AnimateAll whose Duration$ is
// host-bound is refused at lower time, by name, because OverrideAll has no
// host anchor. Other durations (and none) still lower.
func TestAnimateAllHostBoundDurationRefused(t *testing.T) {
	const head = "A:AB$ AnimateAll | Cost$ T | ValidCards$ Creature.YouCtrl | Keywords$ Flying"
	const tail = " | SpellDescription$ x."
	for _, d := range []string{"AsLongAsInPlay", "UntilHostLeavesPlay", "UntilUntaps"} {
		for _, pt := range []string{"", " | Power$ 2 | Toughness$ 2"} {
			rs := lowerSynthRefusals(t, head+" | Duration$ "+d+pt+tail)
			want := "AnimateAll Duration$ " + d + ": host-bound"
			if len(rs) != 1 || !strings.Contains(rs[0].Reason, want) {
				t.Errorf("Duration$ %s pt=%q: refusals %v, want one containing %q", d, pt, rs, want)
			}
		}
	}
	for _, ctl := range []string{" | Duration$ UntilYourNextTurn", ""} {
		if rs := lowerSynthRefusals(t, head+ctl+tail); len(rs) != 0 {
			t.Errorf("control %q refused: %v", ctl, rs)
		}
	}
}
