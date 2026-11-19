package oracle

import "testing"

// The Mana Rig's "{X}{X}{X}, {T}: look at the top X cards" asks for X with
// the most X the pool can pay three times over (CR 107.3, CR 601.2b). Gorge
// offered one more X than dirac did (decision 626 of tail-01 pair 156).
func TestMBCTheManaRigReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_rig_triple_x_offer")
}

// The same ask with a Fetid Heath on the board: the {W/B} filter fee is a
// mana ability the CR 601.2g window can pay (one of its pair's colours),
// and its net +1 is the mana that makes a further X payable (CR 601.2b,
// 605.3b). Gorge's window probe refused the hybrid pip, so its X menu
// stopped one short of the kernel's (decision 444 of tail-01 pair 122).
func TestMBCTheManaRigHybridFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_rig_hybrid_filter_fee_x_bound")
}
