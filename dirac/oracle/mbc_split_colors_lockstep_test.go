package oracle

import "testing"

// CR 709.4 with CR 709.4b: off the stack a split card has both halves'
// characteristics and its colours come from its combined mana cost. Leave //
// Chance ({1}{W} and {3}{R}) in hand is a white and red card, so it is the red
// card Cave-In's alternative cost ("exile a red card from your hand", CR
// 118.9) exiles when it is the only one there. The kernel read the front half's
// colours alone and withheld the cast gorge offered (decision 19 of pool
// tail-73 pair 129: cast(#0{Cave-In},-,16), the hand holding Leave and no
// other red card).
func TestMBCCaveInReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "split_card_hand_colours_pitch")
}
