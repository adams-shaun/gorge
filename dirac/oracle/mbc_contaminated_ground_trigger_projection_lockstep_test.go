package oracle

import "testing"

// Contaminated Ground (CR 305.7, CR 603.2): the recording taps the enchanted
// land for mana, the Aura's "becomes tapped" trigger resolves, and the
// kernel's cast of a four-mana creature then leaves a Contaminated
// Ground-sourced trigger on its stack that gorge's does not hold (decision
// 332 of the recording, from tail-34). The recording is pooled custom: the
// synthetic pools' membership moves with the lowered set.
func TestMBCContaminatedGroundTriggerProjectionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "contaminated_ground_pool_mana_cast_trigger")
}
