package oracle

import "testing"

// TestMBCStingerflingSpiderReprosLockstep replays the synthetic tail-73 game
// the evidence loop recorded (pair 26 of the tail-73-c000 run at land
// 40080f411b): at the main-phase priority p0 holds a hand Stingerfling Spider
// ({4}{G}) with an untapped Shadowblood Ridge ({1}, {T}: Add {B}{R}) — the
// paid filter whose {B}{R} funds the fourth generic — beside an Untaidake,
// the Cloud Keeper, whose legendary-restricted {C}{C} the kernel registered
// as a plain census gap, and an A-Base Camp whose widened restricted
// alternative (any colour, party spells only) widens its plain {C} census
// unit. The kernel's script route dropped the filter walk for that gap's
// sake (scriptOffers' filtOnly condition) and offered nothing for the cast;
// gorge offers cast(#35{Stingerfling Spider},-,0) (divergence at decision
// 773: options options). The same shape as bedlam_reveler_reducecost_x, with
// the filter's fee paid from a widen restricted source's plain {C} census
// alternative: the widened restricted alternative itself stays priced
// per payment (CR 106.6) and funds nothing here.
func TestMBCStingerflingSpiderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "restr_gapother_filter_cast_widen_c")
}
