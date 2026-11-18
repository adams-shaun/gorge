package oracle

import "testing"

// CR 702.2b with CR 615.1: a deathtouch creature whose combat damage is
// prevented (Horn of Deafening on Dire Fleet Ravager) deals no damage, so the
// attacker it blocks takes no deathtouch mark; only the other blocker's real
// damage lands. dirac kept the prevented event's mark and killed the attacker
// that gorge left alive with 1 damage (tail-47-c000 pair 23, decision 474).
func TestMBCDireFleetRavagerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevented_deathtouch_source_marks_no_damage")
}
