package oracle

import "testing"

// Incendiary Oracle's "If a creature dealt damage by CARDNAME this turn
// would die, exile it instead": two Oracles block each other and die to the
// combat damage, and gorge exiles both. The kernel records a source's
// DamagedBy marks only for cards whose filters read F2DamagedBy, and its
// census (prepMcombat) walked triggers, targets and programs but not a
// replacement's own filter, so the Oracle's damage was never marked and both
// went to the graveyard (tail-76 pair 0, decision 98, Cards.Zone).
func TestMBCIncendiaryOracleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_on_leave_damagedby_marks")
}
