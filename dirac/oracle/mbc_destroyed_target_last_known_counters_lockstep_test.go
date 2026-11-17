package oracle

import "testing"

// CR 608.2h: Rite of the Serpent's "if that creature had a +1/+1 counter on
// it" reads the destroyed target's last known counters. The Merfolk
// Secretkeeper carried a +1/+1 counter when it was destroyed, so gorge made
// the Snake; dirac read the counters of the card in the graveyard and made
// none (projection NInst, tail-66-c004 pair 11, decision 770).
func TestMBCWindingCanyonsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "destroyed_target_last_known_counters")
}
