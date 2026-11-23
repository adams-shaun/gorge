package oracle

import "testing"

// CR 605.3b, CR 601.2c: Callous Dismissal ({1}{U}, return target nonland
// permanent) beside an Island and a Sandstone Needle on its last depletion
// counter, whose {R}{R} funds the {1}. The only nonland permanent is the
// player's own Curator's Ward, attached to that Needle, and the payment
// sacrifices the Needle (kfuzz tail-45-c001 pair 6, decision 96).
func TestMBCCallousDismissalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "depletion_land_sacrifice_strands_attached_aura_target")
}
