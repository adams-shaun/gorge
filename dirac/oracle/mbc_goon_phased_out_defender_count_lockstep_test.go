package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it does not
// exist, so it is no creature for Mogg Toady's
// hasFewerCreaturesInPlayThanYou family (Goblin Goon's CantAttack Unless
// defender has fewer creatures in play than you). p1 phased its Ward Sliver
// out, leaving one visible creature to p0's two, so Goblin Goon may attack;
// gorge still counted the phased-out Sliver against the defender and held
// the Goon back.
func TestMBCGoblinGoonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "goon_phased_out_defender_count")
}
