package oracle

import "testing"

// Two opposing replacements for one death: the dying creature's controller
// orders Kalitas, Traitor of Ghet's dies-to-exile rider beside Draugr
// Necromancer's plain dies-to-exile. Only the chosen replacement applies (CR
// 616.1), so choosing Draugr's leaves no Zombie and choosing Kalitas's runs
// the rider. The kernel once refused any order holding a rider and stalled
// (tail-18-c000 pair 59, seed 855460, decision 481; StallExileRider).
func TestMBCExileriderOrderedBesidePlainLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exilerider_ordered_beside_plain")
}
