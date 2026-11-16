package oracle

import "testing"

// CR 601.2g/601.2h, CR 605.3a: an Eldrazi Spawn token under Path of
// Annihilation has two mana abilities, "{T}: Add one mana of any colour" and
// "Sacrifice this creature: Add {C}". With every land tapped, Larger Than Life
// ({1}{G}) is castable from that one token alone: tap it for {G}, then
// sacrifice it for {C}. The two engines disagreed on whether to offer the
// cast (decision 271; dr-mbc-babb pair 198).
func TestMBCLargerThanLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_and_sac_mana_from_one_creature")
}
