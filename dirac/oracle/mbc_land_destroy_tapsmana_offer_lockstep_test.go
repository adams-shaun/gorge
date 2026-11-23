package oracle

import (
	"testing"
)

// CR 405.6c/603.2 with CR 605.4a: Price of Glory's "Whenever a player taps a
// land for mana, if it's not that player's turn, destroy that land" resolves
// after the payment that caused it and its only effect is to destroy the
// permanent whose tap already produced its mana, so the tapped land stays an
// ordinary payment unit (pay.PaymentPlanTapDestroySelfTrigger, the
// destroy-self row of the planner's tap-consequence classification). Pair 15
// of pool tail-70 at land fd13a69db1: on p1's turn, p0's floating pool
// {3}{U}{B}{R}, an Island and a Treasure token pay Port of Karfell's
// "{3}{U}{B}{B}, {T}, Sacrifice this: Mill four, return a creature card from
// your graveyard to the battlefield tapped" -- every land tap triggers p0's
// own Price of Glory, and gorge deferred each tapped land as
// "source:interference:Price of Glory", priced the activation
// unsupported/census_incomplete and dropped it from the priority offer, while
// the kernel offered activate(Port of Karfell,-,1) (decision 852, the
// `options options` class holding back 1 owed card).
func TestMBCPortOfKarfellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "land_destroy_tapsmana_offer")
}
