package oracle

import "testing"

// TestMBCBedlamRevelerReprosLockstep replays the synthetic tail-73 game the
// evidence loop recorded: at the last priority p0 holds a hand Bedlam Reveler
// ({6}{R}{R}, ReduceCost X = the graveyard's instants/sorceries, two there)
// with an untapped Shadowblood Ridge ({1}, {T}: Add {B}{R}) — the paid filter
// whose {B}{R} makes the reduced {4}{R}{R} payable — beside an Untaidake, the
// Cloud Keeper, whose legendary-restricted {C}{C} the kernel registered as a
// plain census gap. The kernel's script route dropped the filter walk for
// that gap's sake (scriptOffers' filtOnly condition) and offered nothing for
// the cast; gorge offers cast(#3{Bedlam Reveler},-,0) (divergence at decision
// 453: options options).
func TestMBCBedlamRevelerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bedlam_reveler_reducecost_x")
}
