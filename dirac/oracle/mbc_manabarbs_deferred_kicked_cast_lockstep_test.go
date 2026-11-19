package oracle

import "testing"

// Manabarbs' TapsForMana damage rider defers every land in gorge's payment
// census (pay.PaymentPlanAbilityTier's foreign watcher row), so gorge's
// autopay surface prices no plan for the kicked Scorch Rider and does not
// offer it. The kernel's potential bound counted the deferred lands as
// ordinary units and offered the kicked cast (tail-34 pair 2, decision 403:
// the kernel listed cast(#65{Scorch Rider},-,1), gorge only pass).
func TestMBCScorchRiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "manabarbs_deferred_kicked_cast")
}
