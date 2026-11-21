package oracle

import "testing"

// CR 603.3c with CR 400.6a: an attack trigger resolves even after its source
// left the battlefield, reading "the player or planeswalker it's attacking"
// from last-known information. Fathom Fleet Swordjack attacked seat 1, then
// Venomspout Brackus's activated ability killed it before the trigger
// resolved: gorge's DealDamage still damages the defending player bound as
// the trigger was queued (TriggeredDefender.Opponent), while dirac stalled
// loudly at its OpAtkPWGuard (StallDefenderPW) because the trigger's attacker
// had no slot to read the attack from (dr-mbc-abbb pair 130).
func TestMBCDrMbcAbbbReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unmodelled_defenderpw")
}
