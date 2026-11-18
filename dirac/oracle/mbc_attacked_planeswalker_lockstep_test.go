package oracle

import "testing"

// CR 508.5 and CR 120.3c: Fathom Fleet Swordjack's attack trigger deals damage
// "to the player or planeswalker it's attacking". Attacking Jaya, Venerated
// Firemage, gorge hits the planeswalker (Valid Planeswalker.TriggeredDefender,
// DefenderObj); the kernel stalled on it (StallDefenderPW; dr-mbc-abbb pair 44).
func TestMBCDrMbcAbbbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attacked_planeswalker_takes_triggered_defender_damage")
}
