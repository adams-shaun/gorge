package oracle

import "testing"

// CR 107.4f: Lashwrithe's equip costs {B/P}{B/P}. At life 2 with an empty
// pool, gorge pays one pip with two life and needs one {B} from a source.
// The only such source is Hall of Tagsin's "{1}, {T}: Add one mana of any
// color", a mana ability with its own mana cost, which gorge's planner
// census does not price: its offer walk finds no witness and withholds the
// activation, and the potential query's lowering dead-ends on the two-life
// face (oracle/gorge.go, "the play is not one gorge makes"). The kernel's
// Phyrexian activation gate counted that census-gap source, offered the
// equip, and the lists parted at decision 943 of this recorded tail-39 game.
func TestPhyrexianActivationCensusGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_census_gap")
}
