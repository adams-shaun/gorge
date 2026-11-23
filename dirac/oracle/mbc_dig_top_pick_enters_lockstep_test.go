package oracle

import "testing"

// TestMBCDigTopPickEntersLockstep replays the synthetic tail-74 game where
// Turntimber Symbiosis digs seven, puts the chosen creature card (Fume Spitter)
// on top of the library and then moves the remembered card onto the
// battlefield with three +1/+1 counters (its mana value is 3 or less). gorge
// put it onto the battlefield; the kernel left it on top of the library
// (divergence at decision 535: projection Cards[40].Pos).
func TestMBCDigTopPickEntersLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dig_top_pick_enters_with_counters")
}
