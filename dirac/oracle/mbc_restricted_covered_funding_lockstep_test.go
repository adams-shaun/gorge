package oracle

import "testing"

// CR 106.6: a source whose only mana is spend-restricted away from the cast
// floats mana the play refuses, so gorge's prefix search prunes it as a dead
// funding branch. Myr Reservoir ("Spend this mana only to cast Myr spells or
// activate abilities of Myr") beside Salvaged Manaworker and Hall of Tagsin,
// two {1} any-colour filters, is the Tribute to the World Tree {G}{G}{G} the
// kernel's search spent its 160 nodes on Reservoir's branches before
// reaching (decision 157 of tail-39-c000 pair 31): gorge offered the cast,
// the kernel did not.
func TestMBCTributeToTheWorldTreeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restricted_covered_funding_is_dead_branch")
}
