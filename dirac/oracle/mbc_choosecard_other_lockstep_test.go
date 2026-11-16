package oracle

import "testing"

// CR 603.4 with Wicked Guardian's "another creature you control": the
// trigger's optional ChooseCard filter (Creature.YouCtrl+Other) excludes the
// resolving source. The kernel's pick offered Wicked Guardian itself beside
// the other creature (decision 309 of tail-26-c001 pair 6, options options).
func TestMBCWickedGuardianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "choosecard_other_excludes_source")
}
