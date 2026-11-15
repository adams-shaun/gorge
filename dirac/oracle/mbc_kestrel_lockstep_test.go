package oracle

import "testing"

// Cloudchaser Kestrel's {W}: target permanent becomes white ability is offered
// by the kernel at a priority decision where gorge offers only pass (decision
// 301 of tail-44 pair 1, autopay, seed 1).
func TestMBCCloudchaserKestrelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kestrel_activate_priority")
}
