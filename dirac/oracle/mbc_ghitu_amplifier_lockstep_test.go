package oracle

import "testing"

const ghituKickedRepro = "ghitu_amplifier_kicked_counter_filter_budget"

// Ghitu Amplifier's kicked cast {1}{R} + {2}{U} (pool tail-01, surface autopay,
// the recorded game; decision 462): player 1's untapped sources are three
// Plains-or-Mountain basics beside Tendo Ice Bridge (a charge-counter
// any-colour ability, the only blue), Fetid Heath and Sunscorched Divide, so
// the cost is payable under CR 601.2h. gorge's prefix search walks its 160-node
// budget (potentialScriptNodes) across the three gap sources, dies before a
// script for the kicked cast is found and withholds it as census_incomplete,
// offering only the plain cast. The kernel's potential arm used to price the
// kicked cast from its units census; a kicked total beside a filter and
// another gap kind now takes the same budgeted search (kw_multikicker.go,
// filtScript), so the recorded game replays clean and the card is unrefused.
func TestMBCGhituAmplifierReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, ghituKickedRepro)
}
