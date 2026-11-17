package oracle

import "testing"

// Foundry Assembler's Improvise payment sacrifices an Eldrazi Scion for mana
// while the arm saves the seat's control mask; the sacrificed token must not
// return to that mask as a stale attacker (CR 111.7, 302.6, 508.1a). The
// game is the tail-45 pair 6 one that first parted at Necrogen Censer.
func TestMBCNecrogenCenserReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_necrogen_censer_attack")
}
