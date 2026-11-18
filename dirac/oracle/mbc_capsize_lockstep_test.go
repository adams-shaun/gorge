package oracle

import "testing"

// Capsize's buyback cast (CR 702.27a) is a {1}{U}{U}+{3} offer; the seat holds
// Island, Island, Plains and a Mage-Ring Network with two storage counters,
// which pays five mana at most, so the offer's affordability is what the two
// engines disagree on (tail-55 pair 2, decision 173).
func TestMBCCapsizeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_buyback_offer")
}
