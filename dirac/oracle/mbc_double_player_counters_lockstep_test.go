package oracle

import "testing"

// CR 122.1 with Aetheric Amplifier's "double the number of each kind of
// counter you have": the controller's energy counters double (2 -> 4 here),
// so Scrapper Champion's "may pay {E}{E}" is still payable on its second
// attack. The kernel left the player's counters alone, offered only the
// decline, and gorge offered pay and decline (tail-68-c002 pair 6, decision
// 414).
func TestMBCScrapperChampionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "double_counters_doubles_player_energy")
}
