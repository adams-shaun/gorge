package oracle

import "testing"

// CR 702.33 with CR 603.2: a kicked Blink of an Eye cast while two Merfolk
// Falconers are out fires each one's "whenever you cast a kicked spell", so
// the caster orders the two triggers. The kernel read a Card.kicked filter on
// a spell as false (a spell is no permanent), queued neither trigger and went
// straight to priority (decision 554 of tail-56 pair 334).
func TestMBCBlinkOfAnEyeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_cast_trigger_watches_other_spells")
}
