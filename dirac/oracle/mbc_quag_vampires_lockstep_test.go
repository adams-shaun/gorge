package oracle

import "testing"

// CR 101.2 / 601.2: the opponent's Iona, Shield of Emeria chose black as it
// entered, so its controller's opponent can't cast black Quag Vampires. The
// kernel's matchSpellAlt left the CantBeCast filter's ChosenColor unread and
// still offered the cast (tail-43-c000 pair 42, decision 540 of 540).
func TestMBCQuagVampiresReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "iona_chosen_color_blocks_cast")
}
