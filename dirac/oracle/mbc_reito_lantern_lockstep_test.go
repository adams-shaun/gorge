package oracle

import "testing"

// CR 605.3b + CR 118.3: Phyrexian Tower, animated into a 2/2 land creature by
// an earlier earthbend, can pay its own "{T}, Sacrifice a creature: Add {B}{B}"
// by sacrificing itself. That mana, with Raging Ravine's {R} or {G}, pays
// Reito Lantern's {3} activation. The kernel did not count a
// sacrifice-a-creature mana ability that names its own source, so it withheld
// the activation gorge offered (tail-73 pair 108, decision 918).
func TestMBCReitoLanternReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reito_lantern_selfsac_mana")
}
