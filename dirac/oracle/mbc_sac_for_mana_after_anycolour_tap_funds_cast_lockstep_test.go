package oracle

import "testing"

const pucaSCovenantRepro = "sac_for_mana_after_anycolour_tap_funds_cast"

// CR 605.3b, 601.2g: with The World Tree (nine lands, six or more) granting the
// untapped Sunlit Marsh "{T}: Add one mana of any color" and Rain of Filth
// giving every land "Sacrifice this land: Add {B}", tapping the Marsh for {G}
// and sacrificing two lands would pay Puca's Covenant's {2}{B} alongside
// Snarespinner's {1}{B} (decision 546, tail-08 pair 13). gorge's prefix search
// (potentialPrefixScript) spends its 160-node budget walking the sacrifice
// answers and finds no script over those grants, so it offers neither cast.
// The kernel's grant search used to keep the grant lists it read at the root
// through a sacrifice step and tapped the Marsh beneath the sacrificed Tree,
// offering the casts; it now re-reads each source's granted abilities at the
// sacrifice steps (mana_grantcost.go sacBranch and tapSacBranch), so it cannot
// pay below the sacrifices either and withholds them like gorge. The recorded
// game replays clean and the card is unrefused. Snarespinner itself is not
// refused: it is evidenced in std-fdn/fdn and the recorded divergence passes
// with Puca's Covenant alone.
func TestMBCPucaSCovenantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, pucaSCovenantRepro)
}
