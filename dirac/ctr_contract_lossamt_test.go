package dirac

import "testing"

// TestContractLossAmtIsUnique pins that the loss copy's abDelayed marker is no
// other abDelayed entry's trigAmt: it once shared -6 with the Ring emblem's
// loot, so resolveDelayed handed every level-2 loot to the loss copy.
func TestContractLossAmtIsUnique(t *testing.T) {
	others := map[string]int16{
		"speed": speedTrigAmt, "venture": initTrigAmt, "monarch draw": monDrawAmt, "monarch steal": monStealAmt,
		"rad drain": radDrainAmt, "ring loot": ringLootAmt, "ring block": ringBlockAmt, "ring drain": ringDrainAmt,
		"extra turn": exTurnRiderAmt, "delayed-planner track": dpTrackAmt,
	}
	for name, v := range others {
		if v == contractLossAmt {
			t.Errorf("contractLossAmt %d is also the %s trigAmt", contractLossAmt, name)
		}
	}
}
