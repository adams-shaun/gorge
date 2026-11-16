package oracle

import "testing"

// CR 700.8: Cascade Seer (a Wizard) beside Game-Trail Changeling (every
// creature type) is a party of two, so its scry 2 orders two cards. The
// matchup's subtype table kept none of the four party roles (nothing named
// them in a filter), so the kernel saw the Seer fill no role and counted a
// party of one, offering one card to arrange where gorge offered two
// (decision 203, pair 350 of tail-39).
func TestMBCDominateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "party_roles_kept_in_matchup")
}
