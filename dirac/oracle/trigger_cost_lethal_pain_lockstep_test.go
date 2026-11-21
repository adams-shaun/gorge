package oracle

import "testing"

// CR 119.4 with CR 118.3: Genesis's upkeep trigger asks {2}{G} with Mana
// Confluence the payer's only untapped source and the payer at 1 life. gorge's
// trigger-cost window taps the source (its one life is payable, CR 119.4), the
// charge fails and the payer, at 0 life, loses. Dirac's window walk refused a
// life-paying source at life <= its cost (the unless / ward gate) and the seat
// lived (tail-12 pair 188 seed 999331, decision 1064 of 1064). The recording is
// the unshrunk game: the shrinker cannot keep the lethal pay.
func TestTriggerCostWindowLethalPainSourceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_window_lethal_pain_source")
}
