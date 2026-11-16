package oracle

import "testing"

// CR 603.3b, CR 605.1b with CR 601.2g: the lands that pay Quicksilver Amulet's
// {4} are tapped for mana one by one (gorge's prefix search activates the
// Plains, the Plains, the Island and Vivid Creek's counter-cost ability before
// the play), so Price of Glory's four tap triggers all wait for priority and
// are placed below the Amulet's ability. The kernel's prefix stopped after the
// Island and tapped Vivid Creek while paying, which put that trigger above the
// Amulet (decision 441 of tail-53 pair 4: Stack[5].Kind).
func TestMBCQuicksilverAmuletReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ctr_source_tapped_before_the_play")
}
