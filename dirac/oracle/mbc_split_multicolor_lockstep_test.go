package oracle

import "testing"

// CR 709.4 with CR 105.2: a split card off the stack has both halves'
// colours, so Farm // Market (white and blue) in a graveyard is a multicolored
// card and a legal target of Vivid Revival ("up to three target multicolored
// cards from your graveyard"). Gorge offered it; dirac's card-side
// PredMultiColor read the front half's colours alone and left it out
// (decision 1262, tail-32 pair 12).
func TestMBCFarmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "split_card_multicolor_in_graveyard")
}
