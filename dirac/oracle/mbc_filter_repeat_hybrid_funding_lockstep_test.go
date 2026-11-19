package oracle

import "testing"

// Orzhova, the Church of Deals's "{3}{W}{B}, {T}: Target player loses 1 life
// and you gain 1 life" beside Mystic Gate (a hybrid-fee filter), Vivid Meadow,
// Energy Refractor ("{2}: Add one mana of any color", a repeatable fee-only
// filter) and the lands: Gate's {C} and Meadow's counter-cost {B} beside two
// Plains and a Forest pay the five mana, so gorge offers the activation. gorge's
// census reaches the hybrid Gate before the Refractor and has cleared its
// relaxed and bounded flags, so it funds nothing for the Refractor's fee
// (paymentPlanCensusOf); the kernel funded it without limit, spent its 160-node
// budget on Refractor lines and never reached the Gate-and-Meadow script
// (decision 521 of pair 111, surface autopay).
func TestMBCOrzhovaTheChurchOfDealsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_repeat_hybrid_gap_funding")
}
