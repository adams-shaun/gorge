package oracle

import "testing"

// CR 605.3b with CR 601.2b: a cycled Decree of Justice's "you may pay {X}"
// announces X before the payment, bounded by the mana the payer's sources
// can make. A filter land (Mystic Gate's "{W/U},{T}: Add two mana") makes one
// mana net of its fee, like its plain {T}: Add {C}, so nine untapped sources
// fund X up to 9, not 10 (decision 1142 of 1142 in the recording).
func TestMBCDecreeOfJusticeFilterLandReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigx_filter_land_bound")
}
