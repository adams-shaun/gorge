package oracle

import "testing"

// CR 601.2b, 601.2f: Esior's raise makes Blaze's price target-dependent, and
// gorge's potential planner then prices an {X} cast from its printed census at
// the smallest X only ("unsupported cost:x" otherwise: no census fallback, no
// prefix search). Song of Freyalise's creature mana is a grant, not a census
// source, so with only Mountains tapped out Blaze is not offered at decision 415
// of dr-mbc-bbab pair 3; dirac once offered it by tapping a granted creature.
func TestMBCBlazeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blaze_granted_mana_creature")
}
