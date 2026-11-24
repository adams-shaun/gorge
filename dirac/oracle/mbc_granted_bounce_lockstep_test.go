package oracle

import "testing"

// CR 605.3b with CR 602.2: Mercurial Pretender enters as a copy of Flamestick
// Courier and carries the granted "{2}{U}{U}: Return this creature to its
// owner's hand". With one Island untapped, three Mountains, a Sandstone Needle
// and a Signpost Scarecrow ("{2}: add one mana of any colour") the second {U}
// is the Scarecrow paid with two red, so the bounce is affordable and priority
// offers it (tail-45 pair 220, seed 2, decision 556).
func TestMBCMercurialPretenderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_bounce_funded_by_fee_filter")
}

// CR 602.2 with CR 605.3b: the same board, but the game PAYS the bounce. At
// the activation seat 0 holds an untapped Island, four Mountains (two spare
// beyond the price), a Signpost Scarecrow and a Sandstone Needle with a
// depletion counter left, beside the Pretender copy of Flamestick Courier, and
// nothing is floating: the policy picks only the one canonical activation of
// the granted "{2}{U}{U}: Return this creature to its owner's hand". Both
// engines' payment planners then choose the sources: the Needle's {R}{R}
// (a depletion counter removed) pays the Scarecrow's {2} filter to make the
// second {U}, the Island makes the first, and two Mountains pay the generic
// {2}. The copy returns to hand (battlefield to hand), so the fee-filter
// payment search runs through a paid activation, not only the offer. The whole
// recorded prefix must be served, so a drifted recording fails.
func TestMBCMercurialPretenderBouncePaidReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "granted_bounce_paid_through_fee_filter")
}
