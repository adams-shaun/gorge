package oracle

import "testing"

// Dungeon Descent's "{4}, {T}, Tap an untapped legendary creature you control"
// is an activated ability, so Battery Bearer's granted "{T}: Add {C}. This
// mana can't be spent to cast a nonartifact spell" pays it: Island, Forest,
// Daring Sleuth and Battery Bearer cover {4} while Zuri is the tapped
// legendary creature. Gorge offered the activation; dirac left it out.
func TestMBCDungeonDescentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_restricted_mana_pays_ability_cost")
}
