package oracle

import "testing"

// CR 112.2a with CR 117.3a: a card with Flashback {2}{U} in its graveyard may
// be cast at priority whenever its controller can pay {2}{U}. The kernel
// offered cast(#45{Desperate Ravings},-,2) — the flashback — at the
// tail-64-c000 pair 160 priority decision while gorge withheld it
// (divergence at decision 990 of the seed-8709783868073162244 game), the same
// payment-plan census class as tapanother_census_one_unit_cast_offer: the
// kernel priced the flashback cast payable where the mana window did not
// support it. The game still diverges at the ticket's land a55bae45f5 and at
// 380060b39b's parent, and replays clean since 380060b39b.
func TestMBCDesperateRavingsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "desperate_ravings_flashback_cast_offer")
}
