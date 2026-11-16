package oracle

import "testing"

// CR 702.106 (converge) with CR 601.2g: Engineered Explosives enters with a
// charge counter per colour of mana spent to cast it. p1 paid {2} generic
// from a Quirion Elves (mana of any colour) and a Wall of Roots, both green,
// so the cast spent one colour and the Explosives entered with 1 charge
// counter (gorge, decision 781 of the recording). The kernel's cast window
// coloured the Elves white — its no-owed fallback (the lowest colour of the
// source's ask order) instead of the colour gorge's payment used — counted
// two colours and entered 2 charge counters. Dwarven Hold, the loop's other
// implicated card, reads the same projection (Cards[i].Counters[k]).
//
// pool dr-mbc-bbaa (focus_00189_UG vs focus_00208_UR, full surface), pair 184
// at land a2df03bb00; the recorded decks replay under the committed pool.
func TestMBCDwarvenHoldReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_window_anycolour_colour_pick")
}
