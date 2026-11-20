package oracle

import "testing"

// CR 601.2f, 702.33a: with Plains, Sunken Ruins, Prismatic Lens and Patriar's
// Seal untapped, Timely Interference ({U}, kicker {1}{R}) is payable kicked
// (Seal makes {U}, Plains pays Prismatic Lens's {1} for {R}, Sunken Ruins adds
// {C}). Gorge offers the kicked cast; dirac once offered only the plain one
// (tail-23 pair 16, decision 179).
func TestMBCTimelyInterferenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicker_cast_filter_chain_afford")
}
