package oracle

import "testing"

// CR 616.1 with CR 119.4: two Worships (identical "damage that would reduce
// your life total to less than 1 reduces it to 1 instead" floors) compete for
// one combat-damage event on the same player. Whichever applies first leaves
// the other with nothing to do, so every order yields the same event and no
// order is asked. Gorge once asked the affected player to order them
// (decision 699 of the tail-55 pair 4 game, implicating Northern Paladin) while
// the kernel, which takes the floors in scan order, went straight to priority.
func TestMBCNorthernPaladinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "two_worships_floor_order_not_asked")
}
