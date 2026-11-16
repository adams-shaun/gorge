package oracle

import "testing"

// An Eldrazi Spawn token under Path of Annihilation has two mana abilities,
// the granted "{T}: Add one mana of any colour" and its own "Sacrifice this
// creature: Add {C}", and is p0's only creature. With every land tapped,
// Larger Than Life ({1}{G}) is paid only by tapping the Spawn for {G} (the
// granted ability, gorge's census gap, so its prefix search) and sacrificing
// it for {C}: the plan sacrifices the spell's only target. gorge's
// scriptSearch.priced replays the witness and withholds a play it leaves
// without a target (witnessReachesFrom, CR 601.2c); dirac's grant search
// offered the cast (decision 271; dr-mbc-babb pair 198).
func TestMBCLargerThanLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_and_sac_mana_from_one_creature")
}
