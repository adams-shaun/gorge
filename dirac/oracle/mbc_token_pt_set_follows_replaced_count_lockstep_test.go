package oracle

import "testing"

// CR 614.1 with CR 111.1: Rootwire Amalgam's "X/X Golem" mint (TokenPower$ X)
// under Adrix and Nev, Twincasters' "twice that many tokens are created"
// makes two Golems, and each enters with the X/X set. The kernel once stalled
// on `tokenpt` because the replacement's count (2) was not the op's amount (1),
// though every doubled token takes the set.
func TestMBCTail22ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "token_pt_set_follows_replaced_count")
}
