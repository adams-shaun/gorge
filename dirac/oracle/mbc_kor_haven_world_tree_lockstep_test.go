package oracle

import "testing"

const korHavenWorldTreeRepro = "world_tree_granted_mana_kor_haven_census"

// CR 605.3b, 601.2g: Kor Haven's census over The World Tree's granted any-colour
// mana beneath Rain of Filth's sacrifices. The kernel's grant search used to keep
// the grant lists it read at the root through a sacrifice step and priced the
// plays below the sacrificed World Tree; it now re-reads each source's granted
// abilities at the sacrifice steps (mana_grantcost.go sacBranch and
// tapSacBranch), like gorge, so the recorded game replays clean and the card is
// unrefused.
func TestMBCKorHavenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, korHavenWorldTreeRepro)
}
