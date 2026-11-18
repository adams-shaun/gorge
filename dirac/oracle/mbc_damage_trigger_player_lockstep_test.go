package oracle

import "testing"

// CR 603.2 with CR 120.3: "that player" in a damage trigger is the player the
// damage was dealt to, not the damage source's controller's opponent. Apprentice
// Sorcerer's controller pinged themself, and the enchanting Sigil of Sleep's
// "return target creature that player controls" must offer only the Sorcerer
// controller's own creatures (dr-mbc-bbab pair 189, decision 509 of 509). The
// kernel read the source controller's opponent and offered the opponent's
// creatures instead.
func TestMBCApprenticeSorcererReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_trigger_player_is_damaged_own_seat")
}
