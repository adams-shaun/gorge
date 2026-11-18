package oracle

import "testing"

// CR 118.3, 605.3a: Mana Vault's upkeep trigger asks for {4} while its
// controller holds a Forest, a Mountain and Llanowar Envoy ("{1}{G}: Add one
// mana of any color", no {T}, repeatable). gorge's trigger-cost gate prices the
// Envoy's colour unbounded and offers "pay"; the payer elects it, the manual
// window taps both lands and filters the red and green into white, the charge
// fails, and the white floats. The kernel left the red and green unfiltered
// (decision 212, Pool[1]: gorge W=1, kernel R=1 G=1; synth tail-28 pair 160 at
// 0fe48693d5), the same cause the Energy Storm repro pins.
func TestMBCManaVaultFilterWindowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_filter_mana_window")
}
