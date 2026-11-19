package oracle

import "testing"

// CR 726.2: "Whenever one or more creatures a player controls deal combat
// damage to the player who has the initiative, the controller of those
// creatures takes the initiative." Battletide Alchemist's optional "you may
// prevent X" parks the player-targeted combat hit behind a replacement-order
// ask, so the hit lands in finishChosenDamage, not in runCombatAssignments'
// synchronous loop -- and runCombatAssignments' tail adjudication saw no
// candidate: the holder (seat 0) kept the designation while the kernel
// correctly moved it to the dealing player's controller (seat 1), which also
// queued the CR 726.2 venture into Undercity (tail-76 pair 7, seed 272187,
// decision 915, "projection Initiative").
func TestMBCBattletideAlchemistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "initiative_handoff_after_parked_prevention")
}
