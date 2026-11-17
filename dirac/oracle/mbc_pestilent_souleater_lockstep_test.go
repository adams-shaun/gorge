package oracle

import "testing"

// CR 107.4f, 118.3: Pestilent Souleater's {B/P} activation is payable with two
// life when every land is tapped. With a granted mana ability live (grantCap),
// the kernel's ungranted census (ungrAvail) carried no life budget, so a
// Phyrexian pip read as unpayable and the activation was dropped at decision
// 594 where gorge still offered activate(#30) (tail-64-c019 pair 3).
func TestMBCPestilentSouleaterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_activate_offer")
}
