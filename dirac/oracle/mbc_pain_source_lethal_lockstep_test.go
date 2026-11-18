package oracle

import "testing"

// CR 605.1a with CR 119.1: Breath of Darigaaz's kicked {3}{R} needs all four
// of the controller's untapped mana sources, one of them Cabal Pit ({T}: Add
// {B}, dealing 1 damage to its controller), and that controller is at 1 life.
// gorge withholds a source whose pain would be lethal from the cast's
// affordability, so the kicked cast is not offered; the kernel offered it
// (decision 372 of 372, dr-mbc-baab pair 182, autopay).
func TestMBCBreathOfDarigaazReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_pain_source_lethal")
}
