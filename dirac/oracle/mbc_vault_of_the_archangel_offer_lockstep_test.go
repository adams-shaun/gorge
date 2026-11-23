package oracle

import "testing"

// Vault of the Archangel's {2}{W}{B},{T} activation: the priority offer must
// list it in both engines exactly when the controller's mana can pay it
// (dr-ev tail-34-c000 pair 2, decision 591 of 591).
func TestMBCVaultOfTheArchangelOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vault_of_the_archangel_activation_offer")
}
