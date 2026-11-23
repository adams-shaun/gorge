package oracle

import "testing"

// CR 608.2h with CR 603.12: Yannik, Scavenging Sentinel exiles Wall of Faith
// while Setessan Training makes its power 1. The reflexive "distribute X
// +1/+1 counters" ability reads the exiled creature's last known power (1),
// so it is put on the stack and asks for targets; the Aura falls off after the
// exile, and a read of the exiled card's own power (0) drops the ability.
func TestMBCTail05ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exiled_creature_lki_power_counters")
}
