package oracle

import "testing"

// CR 601.2f, 700.8: Deadly Alliance costs {1} less per creature in its
// controller's party, so with Black Panther, Most Dangerous (a Human Warrior)
// and four untapped lands p1 can cast it for {3}{B}. gorge offered the cast;
// the kernel did not (decision 152, tail-08 pair 105): the matchup held no
// Warrior subtype, so the party was empty. kgen now keeps the four roles.
func TestMBCDeadlyAllianceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "party_roles_kept_in_matchup_subtypes")
}
