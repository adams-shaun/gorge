package oracle

import "testing"

// Laquatus's Creativity draws X cards then discards X minus the number of
// cards remembered as drawn (RememberDrawn$ AllReplaced). With a Dredge card
// in the drawing player's graveyard each draw is a replacement ask, and an
// answered "draw card" still draws: that card is remembered like an
// unasked draw's. Gorge once left it out, so the discard count was one too
// many (decision 457 of tail-49-c001 pair 17, with Greater Mossdog dredged
// beside it).
func TestMBCBattleMammothReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "draw_remembers_answered_draw_card")
}
