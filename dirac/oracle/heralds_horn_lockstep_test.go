package oracle

import "testing"

// Two Herald's Horns on one battlefield with different chosen types (Echidna and
// Scout); at the upkeep the Scout Horn's reveal of Tireless Tracker (a Scout) is
// answered yes and gorge puts it into the hand. The kernel left it in the
// library (hand 4 vs 3). Recorded as std-trc, Herald's Horn focus, pair 259,
// surface autopay (first found as the std-trc pair-198 Sun Titan divergence).
func TestHeraldsHornSunTitanLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "heralds_horn_two_chosen_types_reveal")
}
