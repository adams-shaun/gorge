package oracle

import "testing"

// CR 107.4f / 601.2b: Dismember's {B/P} pip, announced with only Domesticated
// Watercourse ({T}: Add {U} or {B}) as a black source, offers both faces (pay
// {B}, pay 2 life). The kernel once offered only the life face (kfuzz
// tail-49-c000 pair 2, decision 74); the Taps-deferred source's colour face
// is counted by addPipTapsDefer.
func TestMBCDismemberReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_face_from_dual_combo_land")
}
