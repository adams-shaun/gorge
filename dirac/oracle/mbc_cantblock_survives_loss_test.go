package oracle

import "testing"

// CR 613.11: "Target creature can't block this turn" (Tolaria West's
// Hostile Realm-granted Pump, KW$ HIDDEN CARDNAME can't block.) is a game-rule
// effect on the creature, not an ability it gains, so the later "Creatures
// lose all abilities" (Dress Down, resolved in the same turn) does not end
// it. Sandsteppe War Riders stays unable to block through the loss, and the
// defenders' options must not offer it against Dreamcaller Siren (the kernel
// offered the pair; gorge did not -- lockstep pair 20 of synth pool tail-59,
// seed 14956888561866917009).
func TestMBCSandsteppeWarRidersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cantblock_survives_loss")
}
