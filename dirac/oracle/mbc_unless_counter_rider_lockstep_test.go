package oracle

import "testing"

// CR 118.3 / 605.3b with gorge's unless census: Make Disappear's "counter
// target spell unless its controller pays {2}" resolves against a player whose
// only untapped mana sources are Spire Garden ({T}: Add {R} or {G}) and
// Gemstone Mine ({T}, remove a mining counter: any colour, a bounded rider).
// gorge's unless window lists a mana ability only when its cost is a bare tap
// or a deterministic paid shape (rules unlessWindowUnits; counter removal is
// excluded), so it offers decline alone. The kernel's window counted the
// rider source and offered the pay (decision 595, tail-28 pair 52).
func TestMBCMakeDisappearReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_window_skips_counter_cost_rider")
}
