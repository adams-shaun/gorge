package oracle

import "testing"

// CR 605.3b with CR 118.12: a "counter unless its controller pays {2}" (Silent
// Extraction) is payable when the payer's only untapped lands are Slagwoods
// Bridge ({T}: {R} or {G}) and Desolate Mire ({1},{T}: {W}{B}): the Bridge's
// mana funds the Mire's fee and the Mire's {W}{B} pays the tax. Both engines
// must pose the pay / don't pay choice (decision 435 of 435; tail-75-c001
// pair 62).
func TestMBCSilentExtractionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_fee_land_funded_by_other_land")
}
