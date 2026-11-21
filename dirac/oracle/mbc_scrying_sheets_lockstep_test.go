package oracle

import "testing"

// Scrying Sheets' {1}{S}, {T} activation: gorge offered it at priority (decision
// 355 of tail-71-c000 pair 2) where the kernel did not.
func TestMBCScryingSheetsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scrying_sheets_snow_activation")
}
