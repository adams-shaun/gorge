package oracle

import "testing"

// CR 601.2b: a modal spell announces its mode before the value of X. Confront
// the Past (Charm with Announce$ X) is cast with a planeswalker-less table;
// gorge poses the mode ask first, and the kernel went to the X ask instead
// (decision 386; tail-32-c006 pair 42).
func TestMBCConfrontThePastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "charm_announce_x_mode_first")
}
