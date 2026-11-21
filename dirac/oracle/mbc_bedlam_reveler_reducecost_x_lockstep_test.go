package oracle

import "testing"

// TestMBCBedlamRevelerReprosLockstep replays the synthetic tail-73 game the
// evidence loop recorded: at priority p0 holds a hand Bedlam Reveler with
// instants/sorceries in its graveyard, so its "costs {1} less to cast for
// each instant and sorcery card in your graveyard" static (ReduceCost
// Amount$ X, X = Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn) prices
// the cast affordable. gorge offers cast(#3{Bedlam Reveler},-,0); the kernel
// prices the unreduced {6}{R}{R} and drops the option (divergence at
// decision 453: options options).
func TestMBCBedlamRevelerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bedlam_reveler_reducecost_x")
}
