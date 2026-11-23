package oracle

import "testing"

// Monkey Cage's oracle text is an unconditional "sacrifice Monkey Cage and
// create X 2/2 Monkey tokens". Both players' Cages watched Curiosity Crafter
// enter; the opponent's Cage's four tokens then entered under the first
// player's Cage, whose four new triggers resolved first and sacrificed it. Its
// older Crafter trigger still made four tokens in gorge (the host's remembered
// list survives the resolution) while dirac's per-resolution gate made none
// (tail-75-c000 pair 0, NInst ref=89 kernel=85).
func TestMBCTail75ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "monkey_cage_second_trigger_after_sacrifice")
}
