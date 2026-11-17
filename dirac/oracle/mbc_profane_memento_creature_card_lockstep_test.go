package oracle

import "testing"

// Profane Memento's only trigger is "Whenever a creature card is put into an
// opponent's graveyard from anywhere, you gain 1 life". Boompile's mass destroy
// moved p0's Memento and Boompile (both noncreature permanents) to their
// owners' graveyards in one sweep, and the same sweep also moved p1's Escaped
// Experiment — an artifact creature card — which is the move that matched the
// Memento's ValidCard$ filter. The Memento itself had already left with the
// sweep, so neither engine may put the trigger on the stack (CR 603.6c,
// 603.10). The kernel did.
func TestMBCProfaneMementoCreatureCardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "profane_memento_creature_card")
}
