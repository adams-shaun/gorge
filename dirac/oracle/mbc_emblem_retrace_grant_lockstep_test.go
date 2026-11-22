package oracle

import "testing"

// CR 702.81a with CR 113.1: Wrenn and Six's emblem ("instant and sorcery cards
// in your graveyard have retrace") gives the graveyard You Find Some Prisoners
// its retrace cast, paid by discarding a land card from the hand with the
// floating {R}{R} Chandra made. gorge lists the cast at decision 1054 of pool
// tail-06 (pair 22, surface full); the kernel listed none because only a
// printed Retrace or a card's own retrace static turned on its graveyard
// retrace walk, never a grant carried by an emblem (options options, lead
// gorge, implicates You Find Some Prisoners).
func TestMBCYouFindSomePrisonersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "emblem_retrace_grant_offers_graveyard_cast")
}
