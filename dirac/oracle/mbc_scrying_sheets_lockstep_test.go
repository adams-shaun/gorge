package oracle

import "testing"

// Scrying Sheets' {1}{S}, {T} activation: gorge offered it at priority (decision
// 355 of tail-71-c000 pair 2) where the kernel did not.
func TestMBCScryingSheetsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "scrying_sheets_snow_activation")
	// tail-71-c000 pair 6, decision 740, seed 17421456951445507279, surface
	// full: the same hole in the loop's own game, reached past Wakeroot
	// Elemental's animated Forest -- gorge [pass, activate(#64{Scrying
	// Sheets},-,1)], kernel [pass] at the p1 priority decision.
	stdReproNamed(t, "scrying_sheets_snow_activation_wakeroot")
}
