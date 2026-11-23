package oracle

import "testing"

// CR 106.1 with CR 605.3a and 601.2g: Replicate, the {1}{G}{U} half of
// Repudiate, costs {1} more under Thorn of Amethyst, and Urza, Lord High
// Artificer's tap-another ability adds {U} once per artifact tapped, so the
// caster's untapped artifacts plus a Forest pay it. gorge's potential bound
// counted that repeatable source once and withheld the face the kernel
// offered (decision 445, tail-64 pair 36 seed 231420, surface full). The
// bound now admits the repeats (addPotentialTapRepeat) and both menus offer it.
func TestMBCRepudiateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tapanother_repeat_prices_split_face")
}
