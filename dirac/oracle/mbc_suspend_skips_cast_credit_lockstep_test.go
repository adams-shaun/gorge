package oracle

import "testing"

// CR 702.62a: a suspend special action is never a spell cast, so the cast-time
// credit Inspiring Statuary grants ("Nonartifact spells you cast have
// improvise") cannot pay its {1}{U}. The kernel offered the action and, after
// the planner priced it, gorge asked a convoke/improvise announcement for the
// action -- stranding the game with the card still in hand and two Islands
// floated (decision 464 of pool tail-53 pair 6: gorge at a convoke ask, the
// kernel past a suspend with two TIME counters). The action announces only
// its own X and pays its cost; no Convoke/Harmonize/Improvise/Waterbend or
// delve ask runs.
func TestMBCSuspendSkipsCastCreditReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "suspend_skips_cast_credit")
}
