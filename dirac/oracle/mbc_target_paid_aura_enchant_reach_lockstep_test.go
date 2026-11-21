package oracle

import "testing"

// TestMBCBrilliantWingsReprosLockstep replays the synthetic tail-20 game the
// evidence loop recorded (divergence at decision 407, options options). p0's
// Flash aura Brilliant Wings ({1}{W}, enchant creature you control) is its
// only legal target's own payment: the untapped Eldrazi Spawn token p0's
// cycling of Drownyard Lurker just minted holds the "Sacrifice this creature:
// Add {C}" that funds the cast's {1} beside one untapped Plains. p1's
// Callaphe, Beloved of the Sea has granted its RaiseCost to its creatures and
// enchantments, so the planner declines BW's ordinary witness
// ("shape:target_dependent_cost"), but the cast is payable only by paying with
// a permanent its own announcements need (targetPaidNeedsPlannedPayment), so
// gorge's planner prices it from its cheapest legal target set (targetPricedPlan)
// and offers it; the kernel's targetPaidPlanned answered reaches=true through
// the "no AbSpell means no target to lose" arm while the paying witness had
// just consumed the aura's Enchant target (the Spawn), and withheld the cast.
// A card without an AbSpell is not automatically targetless: an aura's
// Enchant spec is its spell target (tspec).
func TestMBCBrilliantWingsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_paid_aura_enchant_reach")
}
