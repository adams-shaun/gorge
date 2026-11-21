package oracle

import "testing"

// CR 601.2c with Standard Bearer's MustTarget static: an opponent's spell that
// can target a Flagbearer must. Rock Slide ("any number of target attacking or
// blocking creatures", damage divided as you choose) still has a Flagbearer
// among its candidates, so the zero-target answer is withheld. The kernel left
// a divided slot unrestricted and offered done (decision 913 of dr-mbc-baaa
// pair 53).
func TestMBCRockSlideReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "divided_spell_musttarget_requires_flagbearer")
}
