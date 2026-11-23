package oracle

import "testing"

// CR 601.2c-h with gorge's survivorsCanTarget: Aether Gale ("six target
// nonland permanents", {3}{U}{U}) is payable here only by sacrificing the
// three Eldrazi Scion tokens beside the two blue sources, which leaves four
// nonland permanents of the seven it could target. The targets are announced
// before the mana is paid, so the sacrificed tokens still count as targets, but
// gorge refuses a plan that leaves fewer than the slot's minimum standing;
// dirac priced the keep as one surviving target and offered the cast
// (decision 548 of 548, tail-14 pair 99 at main b9a74a26d0).
func TestMBCAetherGaleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_mana_cant_pay_min_targets_spell")
}
