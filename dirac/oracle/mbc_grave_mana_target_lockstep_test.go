package oracle

import "testing"

// CR 601.2c, 601.2g, 605.1a: Banish from Edoras costs {2} less when it targets a
// tapped creature. With {W}{W}{W} floating, Value Town as the one untapped land
// and Jack-o'-Lantern in the graveyard, the untapped Border Guard would price the
// cast at {4}{W}. gorge's window census is incomplete while a graveyard mana
// source can be activated, so its target menu keeps the candidate the pool cannot
// pay; dirac's census ignored the hand and the graveyard and withheld it (decision
// 262, pair 14 of the evidence loop's tail-32-c001).
func TestMBCBorderGuardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grave_mana_source_keeps_unpayable_target")
}
