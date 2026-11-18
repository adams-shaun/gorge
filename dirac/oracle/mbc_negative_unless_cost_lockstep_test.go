package oracle

import "testing"

// CR 107.1b: Mausoleum Wanderer's "Sacrifice CARDNAME: Counter target instant
// or sorcery spell unless its controller pays {X}, where X is CARDNAME's
// power" reads X from the sacrificed Wanderer's last-known power, -2 under
// Torpor Dust's -3/-0, so the unless cost is {0}. gorge clamps it
// (effects.UnlessCostResolved) and offers pay and decline; the kernel priced
// {-2} as unaffordable and offered decline only (tail-53 pair 48, decision 183).
func TestMBCMausoleumWandererReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "negative_unless_cost_amount")
}
