package oracle

import "testing"

// Kyren Negotiations ({T an untapped creature you control}: 1 damage to target
// player or planeswalker): dr-mbc-aaab pair 36, decision 916, the engines part
// at a modes decision for p1 (gorge offers mode 1, the kernel does not).
func TestMBCKyrenNegotiationsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kyren_negotiations_modes")
}
