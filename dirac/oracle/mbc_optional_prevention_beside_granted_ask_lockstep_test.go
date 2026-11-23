package oracle

import "testing"

// CR 616.1 with CR 616.1g: the defender's Commencement of Festivities Effect
// (a combat-damage prevention record) and the attacker's Battletide Alchemist
// (an Optional$ shield) both match one combat hit on the player, so gorge asks
// the affected player the order over both plus the Alchemist's skip. The kernel
// dropped the optional line and dealt the step with no ask (decision 1249 of
// tail-76-c000 pair 2, Galion, Elvenking's Butler attacking).
func TestMBCGalionElvenkingSButlerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "optional_prevention_beside_granted_prevention_ask")
}
