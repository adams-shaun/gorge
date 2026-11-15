package oracle

import "testing"

// Frogmyr Enforcer ({7}, affinity for artifacts, prototype {3}{R} 2/2): at
// decision 136 of dr tail-75 pair 281 the kernel offered the prototype cast
// (castface 32) that gorge did not.
func TestMBCFrogmyrEnforcerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prototype_affinity_castface")
}
