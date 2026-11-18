package oracle

import "testing"

// Titan's Revenge ({X}{R}{R} sorcery): gorge offered the cast at p1's priority
// where dirac did not (decision 143, tail-59-c000 pair 42).
func TestMBCTitanSRevengeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "titans_revenge_x_cast_offer")
}
