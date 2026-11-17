package oracle

import "testing"

// CR 605.1a, 601.2g-h: Nature's Embrace grants the enchanted land "{T}: Add
// two mana of any one color", and both units are that one colour, whichever
// the player chooses. The kernel's census counted the grant as two
// independent any-colour units, so with the land's own {T} sources tapped it
// read Izzet Keyrune's {U}{R} animation as payable and offered it at a
// priority decision gorge withholds (decision 868 of 868, pair 466 of
// tail-19: the land and the Keyrune's other {U} source were both tapped).
func TestMBCEyeOfUginReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_one_colour_census")
}
