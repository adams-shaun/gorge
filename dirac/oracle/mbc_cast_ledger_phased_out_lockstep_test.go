package oracle

import "testing"

// CR 702.26b treats a phased-out permanent as though it does not exist, but a
// spell cast this turn stays a spell cast this turn (CR 601.2i; Forge keeps
// the cast list as cast-time copies and filters it without a phasing check).
// Fae Offering's end-step trigger gates on Count$ThisTurnCast of a creature
// spell and of a noncreature spell: its controller cast Kenrith's
// Transformation, then Guardian of Faith, whose ETB phased out the Aura's
// enchanted token and with it the Aura. Gorge's cast-ledger walk matched the
// phased-out Aura as a battlefield permanent and found no noncreature cast, so
// it dropped the trigger the kernel pushed. The recording is the first
// divergence of dr-mbc-bbaa pair 86: projection NStack at decision 362, gorge
// leading, on the autopay surface.
func TestMBCCastLedgerPhasedOutSpellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_cast_ledger_phased_out_spell")
}
