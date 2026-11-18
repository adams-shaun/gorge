package oracle

import "testing"

// CR 303.4f, 707.2: Calix, Guided by Fate's "create a token that's a copy of a
// nonlegendary enchantment you control" (CopyPermanent Choices$) picks
// Detainment Spell, an Aura. The token enters attached to what its controller
// chooses (here Calix, its one legal host). The kernel stalled copyallask on
// every copied Aura or as-enters choice instead of entering it (pair 5, pool
// tail-31, surface autopay, decision 287).
func TestMBCTail31ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_choices_aura_token")
}
