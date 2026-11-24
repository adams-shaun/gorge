package oracle

import "testing"

const wardscaleCrocodileRepro = "wardscale_crocodile_cast_offer"

// Wardscale Crocodile's {4}{G} cast (CR 601.2g) would be payable on a board
// where Rain of Filth made every land "Sacrifice this land: Add {B}" until end
// of turn (CR 605.3b) and The World Tree lets the one untapped Swamp tap for
// {G}: one World Tree tap plus four land sacrifices (pool tail-08 pair 20, seed
// 933465, decision 721). Gorge's prefix script search
// (engine/rules/potential_script.go) has a 160-state budget
// (potentialScriptNodes) and runs out before it finds one tap and four
// sacrifices among ten interchangeable sacrifice gaps, so its menu withholds the
// cast. The kernel's grant search used to keep the grant lists it read at the
// root through a sacrifice step and offered it; it now re-reads each source's
// granted abilities at the sacrifice steps (mana_grantcost.go sacBranch and
// tapSacBranch), so it withholds the cast like gorge. The recorded game replays
// clean and the card is unrefused.
func TestMBCWardscaleCrocodileReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, wardscaleCrocodileRepro)
}
