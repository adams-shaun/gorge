package oracle

import "testing"

// TestMBCBashToBitsReprosLockstep guards a fix already on main: dirac offered
// a flashback cast of Bash to Bits ({4}{R}{R}) that gorge did not, because the
// Energy Refractor fee-only filter must be able to fund the second R of the
// flashback cost (CR 605.3a mana abilities, CR 601.2g paying costs). The
// recording diverged at decision 373 ("options options") before the fix.
func TestMBCBashToBitsReprosLockstep(t *testing.T) {
	stdReproNamed(t, "bash_to_bits_flashback_refractor")
}
