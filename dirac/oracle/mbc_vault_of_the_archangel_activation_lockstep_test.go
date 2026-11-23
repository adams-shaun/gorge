package oracle

import "testing"

// CR 601.2 with CR 605.1a: Vault of the Archangel's second ability costs
// {2}{W}{B} plus its own tap, and p1 can pay it from the lands on the
// battlefield. gorge offers `activate(#74{Vault of the Archangel},-,1)` at
// decision 250; the kernel offers only pass, so its affordability walk holds
// the activation back (tail-34 pair 25, seed 255781, surface autopay).
func TestMBCVaultOfTheArchangelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vault_of_the_archangel_activation")
}
