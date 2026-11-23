package oracle

import "testing"

// Ainok Survivalist's special option at a priority decision (ev-tail-12-c000
// pair 26, seed 13261916301239590493): gorge offered special(#0{Ainok Survivalist})
// where the kernel offered pass alone.
func TestMBCAinokSurvivalistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ainok_survivalist_special_offer")
}
