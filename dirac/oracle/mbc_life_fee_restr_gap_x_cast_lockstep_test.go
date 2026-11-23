package oracle

import "testing"

// CR 605.3a with CR 601.2g: p1's Untaidake, the Cloud Keeper ("{T}, Pay 2
// life: Add {C}{C}. Spend this mana only to cast legendary spells") is a
// restricted planner unit that the kernel registered as a census gap
// (gfGapOther), and that registration kept p1's fee-carrying filter
// Shadowblood Ridge ("{1}, {T}: Add {B}{R}") off the filter search, so the
// kernel withheld the {X}{X}{G} cast of Orcish Settlers its ridge funds
// (gorge offered the cast; decision 140, tail-73-c000 seed 352851). The same
// census-gap class as life_fee_restr_gap_filter_cast (Geosurge), where the
// withheld play is a plain cast; here it is an X cast. Fixed by 2d110d1e35,
// not by the deferred-tap-extra X window the recording was first filed under
// (reverting that hunk leaves this game clean; reverting 2d110d1e35 re-diverges
// decision 140 with the hunk in).
func TestMBCOrcishSettlersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "life_fee_restr_gap_x_cast")
}
