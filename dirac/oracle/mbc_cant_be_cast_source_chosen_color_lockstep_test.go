package oracle

import "testing"

// CR 604.1 with CR 105.4: Iona, Shield of Emeria named black as it entered,
// so its controller's opponent can't cast black spells. The opponent holds
// The Haunt of Hightower with four Mountains and two Swamps untapped: gorge
// offers no cast of it, while the kernel's CantBeCast filter read the
// ChosenColor from a resolution's ChooseColor answer (there is none at
// priority) instead of the static source's own choice and offered the cast
// (tail-43 pair 46, decision 474).
func TestMBCTheHauntOfHightowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cant_be_cast_source_chosen_color")
}
