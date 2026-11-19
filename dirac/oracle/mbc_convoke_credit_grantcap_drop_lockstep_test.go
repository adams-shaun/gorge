package oracle

import "testing"

// CR 702.51a: gorge's cast-offer gate prices a printed-convoke spell at its
// credited base (castOfferBase: convokeCost, then improviseCost), so the
// priority list carries the plain cast when the credited base is payable by
// the printed census alone -- every land tapped, the creatures paying the
// coloured pips. The kernel's grantCap dropped that cast: its model of
// gorge read the planner's "shape:contribution" refusal (payment_plan.go)
// as the offer verdict, but the refusal only holds for a base the credit
// leaves unpayable (and for the shapes the offer gate does not credit,
// like delve). Worldsoul Colossus {X}{G}{W} at X = 0 beside the tapped
// lands is the pinned game.
func TestMBCWorldsoulColossusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_credit_grantcap_drop")
}
