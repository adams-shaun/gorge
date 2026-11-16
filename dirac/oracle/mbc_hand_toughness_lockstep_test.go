package oracle

import "testing"

// CR 601.2a-c: a spell leaves the hand for the stack before its targets are
// chosen, so Sungold Barrage ("destroy target creature with toughness 4 or
// greater") cast with three other cards in hand can't target a Sturmgeist
// whose toughness is its controller's hand size. tail-41 pair 26.
func TestMBCSungoldBarrageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sungold_barrage_hand_sized_toughness")
}
