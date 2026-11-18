package oracle

import "testing"

// CR 733.2 with CR 602.2: a graveyard ability whose cost the bot pays the
// unpayable way (Purple Pentapus's "tap an untapped creature" picking the
// only black mana source) is reversed with no progress; the SECOND identical
// reversal holds the activation out of the next priority menu. gorge held it
// out, the kernel's graveyard offer (priorityGraveAct) never read the
// hold-out and kept offering it (decision 491 of the tail-62 pair 133).
func TestMBCPurplePentapusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "graveyard_activation_held_out_after_second_abort")
}
