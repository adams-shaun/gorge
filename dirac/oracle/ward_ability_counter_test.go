package oracle

import "testing"

// TestWardCountersTheAbilityNotItsSpell: a ward trigger whose warded stack
// object is an ability (Ugin, Eye of the Storms' cast trigger) of a spell
// that is still on the stack counters that ability, not the spell (CR 702.21a:
// "counter that spell or ability"). Declining the ward left the kernel
// holding the trigger and sending the Ugin spell to the graveyard, where
// gorge keeps the spell and drops the trigger.
func TestWardCountersTheAbilityNotItsSpell(t *testing.T) {
	t.Parallel()
	res := replayStdReproResult(t, synthReg(t), "custom", "ugin_eye_of_the_storms_cast_trigger_after_ward_counter", true)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
}
