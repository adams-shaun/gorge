package oracle

import "testing"

// CR 106.6, 107.3: gorge's manual window lists only the sources whose mana the
// payment can still use. In dr-mbc-baab pair 108 (evl-11190733) Crypt Rats'
// black-only {X} (X = 2) was paid through the window from two Swamps; the
// kernel's window walk took Gilanra, Caller of Wirewood (a deferred
// TriggersWhenSpent$ source) first and floated a {G} the cost could not spend.
// windowPick now skips a deferred source whose colours the clamp does not want,
// as it does a restricted one.
func TestWindowSkipsDeferredSourceColourUnowedReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "window_skips_deferred_source_colour_unowed")
}
