package oracle

import "testing"

// CR 106.6: a floating restricted batch whose restriction names no
// non-spell payment (the Powerstone's CantCastNonArtifactSpells) may pay a
// "counter target spell unless its controller pays {4}" tax -- gorge's offer
// gate prices the pool AvailableFor holds, restricted batches the payment
// admits included (engine/rules/pay/unless.go UnlessCostValuePayable), and
// the kernel's unless settlement (dirac/mana.go payWindow) spends exactly
// those batches. The kernel's offer gate (dirac/interp_ask.go execPay's
// canPayGen/windowPays) left them out, so gorge offered the countered
// spell's controller pay and decline (Flip the Switch: mode 0 and mode 1)
// where the kernel offered decline only -- two floating Powerstone mana and
// two untapped lands reach {4}. gorge leads; the recording is the first
// divergence of a tail-57 focus run (Flip the Switch, Powerstone Shard,
// Circular Logic, Scatter Arc, Thoughtbind, Azorius Charm), pair 121, replayed
// on its own decks' table (pool custom: a tail pool's deal moves as evidence
// is credited).
func TestMBCUnlessOfferCountsRestrictedManaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_offer_counts_restricted_mana")
}
