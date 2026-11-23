package oracle

import "testing"

// CR 611.3, 613: Halvar, God of Battle's Sword of the Realms is re-equipped for
// {1}{W} while Rain of Filth lets every land sacrifice itself for {B} and The
// World Tree gives lands "{T}: Add one mana of any color" only while their
// controller has six lands. gorge's prefix search asks each source afresh at
// every node, so a line that sacrifices The World Tree (or enough lands) finds no
// granted colour below it, and the search spends its budget without a W. The
// kernel's search kept the root's grant lists, branched on the dead any-colour
// grant and priced an equip gorge withholds (tail-08 pair 15, decision 395).
func TestMBCHalvarGodOfBattleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_rereads_grants_after_own_sacrifice")
}
