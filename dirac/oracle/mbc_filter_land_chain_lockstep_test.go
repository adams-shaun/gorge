package oracle

import "testing"

// CR 605.3b with CR 106.1b: a filter land's mana ability can be paid with
// mana from another land, so Island's {U} pays Flooded Grove's {G/U} fee and
// the Grove's {U}{U} is the blue a six- or five-mana blue spell needs. gorge
// offered Deep-Sea Serpent ({4}{U}{U}) and Messenger Drake ({3}{U}{U}) with
// an Island, Flooded Grove, Wooded Bastion and four Plains untapped; the
// kernel offered neither (decision 369, tail-15 pair 30).
func TestMBCFilterLandChainReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_land_chain_blue")
}
