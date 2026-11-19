package oracle

import "testing"

// Racers' Ring's "{2}{R}{G}, {T}, Sacrifice: Draw a card" beside Mystic Gate's
// {W/U} filter and, later in zone order, Energy Refractor's repeatable "{2}:
// Add one mana of any color". gorge's census no longer skips a gap after the
// hybrid filter, so the Refractor's fee funds the prefix search wherever it
// sits in zone order, as dirac's filtFunding does (pool tail-02 pair 183,
// decision 298).
func TestMBCRacersRingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "racers_ring_fee_gap_funding")
}
