package oracle

import "testing"

// CR 702.33a with CR 605.3b: one tap of a one-colour open production ("Add X
// mana of any one color", Lotus Blossom's four petals here) makes all its mana
// one colour, so Keldon Strike Team's kicked cast {3}{R}{W} is no play beside
// two Swamps, a Frostwalk Bastion and it -- the pick covers {R} or {W}, never
// both. The potential census priced the production as four independent
// any-colour units, and the kicked cast's offer arm kept the over-bound answer
// (decision 363 of tail-74-c000 pair 63).
func TestMBCKeldonStrikeTeamReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "potcensus_onepick")
}
