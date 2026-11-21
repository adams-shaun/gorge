package oracle

import "testing"

// CR 605.3a, 601.2g-h: Henge of Ramos's "{2},{T}: one mana of any color" is the
// only red source on the board (A-Riveteers Initiate, Cyclops Superconductor and
// Urabrask's Anointer in hand), so gorge offers all three casts through it; the
// kernel offered none of them (tail-26-c000 pair 60, decision 404).
func TestMBCARiveteersInitiateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "henge_fee_red_casts_offered")
}
