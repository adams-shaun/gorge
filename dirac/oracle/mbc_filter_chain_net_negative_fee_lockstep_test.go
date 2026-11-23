package oracle

import "testing"

// CR 605.3a-b, 602.2b: Orzhova, the Church of Deals' {3}{W}{B},{T} ability is
// payable from Mystic Gate's {W/U},{T}: {W}{W} and Energy Refractor's {2}:
// {B}, with Islands and Forests beneath. The two engines must agree whether
// that chain makes the activation a legal priority option.
func TestMBCOrzhovaTheChurchOfDealsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_chain_net_negative_fee_activation")
}
