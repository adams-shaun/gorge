package oracle

import "testing"

// A four-seat edh pod: p2 loses at 0 life in the same state-based check that
// kills four of its creatures, one of them a Station Spacecraft that is a
// creature only through its STATION 4+ static. Every "whenever a creature
// dies" trigger looks back at all four (CR 603.10a, 704.3); gorge dropped the
// Spacecraft once p2's statics stopped applying with the loss, and offered
// three instances of each trigger to the kernel's four (pod 107 of the edh
// sweep, decision 1741).
const eliminatedPlayersCreaturesDieRepro = "edh_eliminated_players_creatures_die"

// eliminatedPlayersCreaturesDieFirstAt is the decision the divergence was at.
const eliminatedPlayersCreaturesDieFirstAt = 1741

// TestEdhEliminatedPlayersCreaturesDieTriggersLockstep: no divergence past the
// recorded decision.
func TestEdhEliminatedPlayersCreaturesDieTriggersLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, eliminatedPlayersCreaturesDieRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= eliminatedPlayersCreaturesDieFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, eliminatedPlayersCreaturesDieFirstAt)
	}
}
