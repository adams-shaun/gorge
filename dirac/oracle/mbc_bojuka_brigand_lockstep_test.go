package oracle

import "testing"

// CR 616.1: Commencement of Festivities' this-turn "prevent all combat damage
// that would be dealt to players" and Battletide Alchemist's optional "may
// prevent X of that damage" both apply to the attackers' combat damage to the
// Alchemist's controller, so gorge asks that player which applies first, with
// the line's skip (decision 1668 of tail-76-c000 pair 52). The kernel dropped
// the optional line and dealt the damage without the ask; it now poses the
// order with the Optional$ skip (spCombatCands, CR 616.1g), so the recording
// replays lockstep.
func TestMBCBojukaBrigandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scoped_prevention_beside_optional_static_prevention")
}
