package oracle

import "testing"

// The Mana Rig's "{X}{X}{X}, {T}: look at the top X cards" asks for X with
// the most X the pool can pay three times over (CR 107.3, CR 601.2b). Gorge
// offered one more X than dirac did (decision 626 of tail-01 pair 156).
func TestMBCTheManaRigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_rig_triple_x_offer")
}
