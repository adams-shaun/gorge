package oracle

import "testing"

// A "whenever you lose life for the first time each turn" trigger (Forge
// LifeLost FirstTime$ True) fires on the turn's first life-loss event only.
// Two attackers' combat damage reaches the player as two losses, so each of
// the player's two level-2 Intermediate Chirographies triggers once, on the
// first; gorge (trigmatch firstLifeLossThisTurn) offered two triggers to
// order. The kernel had no first-time gate on LifeLost and queued one per
// loss, offering four (decision 700 of tail-13 pair 33). The recording plays
// on the table its own decks need (pool custom): a tail pool's deal moves as
// evidence is credited.
func TestMBCFirstTimeLifeLossFiresOnceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "first_time_life_loss_fires_once")
}
