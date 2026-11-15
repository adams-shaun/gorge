package oracle

import (
	"testing"
)

// A trigger's own pay/decline ask offers "pay" when the payer's potential mana
// can reach the cost (rules respay.go resPayNeedsMana over
// rules potential.go potentialManaFor), and gorge's potential walk folds a
// mana ability whose Amount$ is indeterminate unbounded in every colour
// (potentialAmount) — Dwarven Hold's "{T}, Remove any number of storage
// counters: Add {R} for each" beside Mana Vault's "you may pay {4}. If you do,
// untap this artifact" (MB2). The kernel's potential walk priced the battery
// at its live counters (none) and asked decline only, so the two option lists
// parted (decision 525 of pair 63, pool tail-75, surface autopay).
func TestMBCManaVaultReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_battery_potential_unbounded")
}
