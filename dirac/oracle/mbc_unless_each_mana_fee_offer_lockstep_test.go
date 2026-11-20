package oracle

import "testing"

// TestMBCEldestDragonHighlanderReprosLockstep pins the unless-cost offer on
// Eldest Dragon Highlander's upkeep trigger: its {W}{U}{B}{R}{G} is payable
// because Overflowing Basin's "{1}, {T}: Add {G}{U}" can be funded by Power
// Depot's {C} (CR 605.3b), and gorge offers the pay and decline modes. The
// kernel's greedy unless window priced only its free sources, so it offered
// decline alone (decision 478 of 478).
func TestMBCEldestDragonHighlanderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_each_mana_fee_offer")
}
