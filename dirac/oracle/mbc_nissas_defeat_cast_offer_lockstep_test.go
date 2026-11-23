package oracle

import "testing"

// Nissa's Defeat sat in p0's hand with untapped mana sources but the
// engines disagreed on offering its cast at priority (decision 946 of
// tail-39-c000 pair 19).
func TestMBCNissaSDefeatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "nissas_defeat_cast_offer")
}
