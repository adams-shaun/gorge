package oracle

import "testing"

// CR 702.66a: Delve lets a spell's controller exile cards from their graveyard
// to pay generic mana, so a Shambling Attendants ({7}{B}) is castable once the
// graveyard plus the lands cover its cost. Gorge offered the cast at decision
// 594 of tail-10-c000 pair 30 and dirac did not.
func TestMBCShamblingAttendantsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delve_cast_offer")
}
