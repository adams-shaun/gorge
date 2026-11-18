package oracle

import "testing"

// CR 603.2 / CR 120.3: a damage trigger's "that player" is the player the
// damage went to, even when the source's controller is that player. Matoc,
// Lavamancer's ability can target its own controller; Sigil of Sleep on Matoc
// then returns a creature the controller (not the opponent) controls, so only
// Matoc is a legal target. The kernel named the source controller's opponent
// as the damaged player and offered the opponent's Blightwidow (decision 644
// of dr-mbc-bbab pair 94).
func TestMBCMatocLavamancerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "damage_trigger_names_self_damaged_player")
}
