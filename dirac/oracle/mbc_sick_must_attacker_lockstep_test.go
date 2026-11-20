package oracle

import "testing"

// CR 508.1d: a creature that "attacks each combat if able" binds nothing while
// it is summoning sick (CR 302.6), so a Utvara Scalper cast this turn leaves
// the attackers decision with Done offered beside the other creature's attack
// (decision 672 of tail-71 pair 15).
func TestMBCUtvaraScalperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sick_must_attacker_offers_done")
}
