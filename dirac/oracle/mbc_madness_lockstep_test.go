package oracle

import "testing"

// CR 702.35a: a madness trigger's "yes" (cast the exiled card for its madness
// cost) is offered only when the cast can be paid and targeted. The recording
// is a tail-49 game where Murderous Compulsion was discarded at cleanup (decision
// 518 of the game) and the two engines posed different cast-or-graveyard menus.
func TestMBCMurderousCompulsionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offered_only_while_payable")
}
