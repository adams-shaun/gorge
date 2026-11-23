package oracle

import "testing"

// CR 605.3b, 613.1f: The World Tree's "{T}: Add one mana of any color" grant
// lasts only while its holder is on the battlefield with six lands. With Rain
// of Filth's "Sacrifice this land: Add {B}" on every land, the prefix search
// that prices Goblin Heelcutter's dash cost ({2}{R}) must not tap the last
// Swamp for the any-colour {R} after its own line sacrificed The World Tree:
// gorge asks each source's abilities afresh at every node, so the grant is
// gone below that step. The kernel kept the root's grant list below a granted
// sacrifice step and offered cast(#48{Goblin Heelcutter},-,10) that gorge
// withheld (decision 686 of 686, tail-08-c000 pair 49, seed 20933, surface
// autopay, mulligans 1).
func TestMBCGoblinHeelcutterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "goblin_heelcutter_options")
}
