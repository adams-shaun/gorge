package oracle

import "testing"

// CR 118.12 with CR 605.3b: Conversion's upkeep "sacrifice unless you pay {W}{W}"
// is payable from a Plains and Unknown Shores' "{1},{T}: Add one mana of any
// color" funded by an Island, so gorge offers the pay mode beside the decline.
// dirac offered only the decline (decision 326 of kfuzz tail-34 pair 4).
func TestMBCConversionReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_pay_paid_anycolour_source")
}
