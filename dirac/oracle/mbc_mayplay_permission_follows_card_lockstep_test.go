package oracle

import "testing"

// CR 611.2c with CR 107.5's effect wording: Skyclave Shade's landfall
// trigger (a land enters under your control while it is in your graveyard,
// on your turn) grants "you may cast it from your graveyard this turn" as a
// continuous effect over that card, not an ability gained by the card. The
// grant is not ended by the card leaving the graveyard: cast from the
// graveyard (Forge's ForgetOnMoved$ Exile forgets only on an exile move),
// resolve, then sacrificed to Ravenous Harpy's ability, it is back in the
// graveyard the same turn and the cast stands again. gorge offers it
// (decision 426 of tail-32 pair 6, seed 1, autopay, mulligans 1); the
// kernel's permission record was version-tied, so every zone move killed it
// and the option vanished.
func TestMBCMayplayPermissionFollowsCardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_permission_follows_card")
}
