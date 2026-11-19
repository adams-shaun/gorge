package oracle

import (
	"testing"
)

// CR 107.4 (generic mana is paid with any mana) with CR 605.3b (a mana
// ability's own cost is paid from mana already in the pool): a player facing
// "counter that spell unless its controller pays {2}" may pay with the {U} an
// untapped Island produces funding a Desolate Mire's "{1}, {T}: Add {W}{B}",
// the {W}{B} covering the tax. The gorge offer gate once priced the tax as
// unpayable and offered decline only, while dirac offered the pay branch
// (decision 366 of 366; tail-75-c001 pair 20, evloop evidence game).
func TestMBCSilentExtractionFeeFundedReachReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_fee_funded_reach")
}
