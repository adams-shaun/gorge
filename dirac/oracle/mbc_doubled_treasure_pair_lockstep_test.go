package oracle

import "testing"

// CR 605.3b, 111.10a: Alchemist's Talent at level 2 gives each Treasure "{T},
// Sacrifice this artifact: Add two mana of any one color", so two Treasures
// fund four mana. At the third priority of a turn with only the two Treasures
// untapped (decision 221 of tail-16-c000 pair 81), gorge offered Daru
// Sanctifier's {3}{W}, Soul of Migration's evoke {3}{W} and Sword of Body and
// Mind's {3}; the kernel offered none of them.
func TestMBCDaruSanctifierReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "doubled_treasure_pair_funds_four_mana_casts")
}
