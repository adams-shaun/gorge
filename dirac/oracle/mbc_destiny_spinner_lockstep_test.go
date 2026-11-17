package oracle

import "testing"

// Destiny Spinner's activation ({3}{G}) beside Ashnod's Altar (a sacrifice-any
// mana source) and granted mana abilities: gorge's one prefix search activates
// the Altar and the grant holders as steps of one search, and offers the
// activation; the kernel split them across two searches and left it out.
func TestMBCDestinySpinnerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_any_gap_beside_grants_activation")
}
