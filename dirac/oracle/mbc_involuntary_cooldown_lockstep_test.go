package oracle

import "testing"

// Involuntary Cooldown ({3}{U} sorcery, up to two targets) is offered by gorge
// at priority where dirac offers only pass (tail-74-c000 pair 6, decision 139).
func TestMBCInvoluntaryCooldownReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "involuntary_cooldown_not_offered")
}
