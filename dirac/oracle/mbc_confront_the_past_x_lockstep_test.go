package oracle

import "testing"

// CR 601.2b-c: Confront the Past's X is announced after its mode is chosen, and
// an X at which the chosen mode has no target (Return: a planeswalker card with
// mana value X or less in your graveyard, here Tevesh Szat's 5) cannot be
// announced. gorge offered X 5..7; the kernel offered 0..7 because its target
// feasibility read every mode, not the chosen one.
func TestMBCConfrontThePastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "confront_the_past_x_chosen_mode")
}
