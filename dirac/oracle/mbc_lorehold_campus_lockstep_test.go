package oracle

import "testing"

// Lorehold Campus's "{4}, {T}: Scry 1." was offered by gorge at a priority
// decision where dirac offered only pass (decision 269 of pair 29, tail-16-c000).
func TestMBCLoreholdCampusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lorehold_campus_scry_offer")
}
