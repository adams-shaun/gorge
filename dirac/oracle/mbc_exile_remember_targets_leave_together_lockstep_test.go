package oracle

import "testing"

// CR 603.10a: Brago, King Eternal exiles Opal Lake Gatekeepers together with
// the Valor of the Worthy enchanting it (and tokens), as one simultaneous
// departure, so Valor's "when enchanted creature leaves the battlefield"
// trigger still sees the Gatekeepers go and is offered in the trigger order
// (decision 498 of tail-23-c000 pair 6). The kernel ran each exile target on
// its own, so Valor was already gone when the Gatekeepers' leave was checked
// and only gorge offered Valor's trigger.
func TestMBCValorOfTheWorthyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "exile_remember_targets_leave_together")
}
