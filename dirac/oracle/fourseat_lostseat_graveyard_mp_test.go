package oracle

import "testing"

// A seat that loses in a state-based pass leaves the game only at the pass's
// end (CR 704.3, 800.4a: gorge's sweepDepartedPlayers). Until then its
// graveyard still holds its cards, so a count over the opponents' graveyards
// still includes them while that pass decides which creatures die.
//
// Both recordings are full 100-card cmd5 pods, Wretched Ranks four times
// (kfuzz-style lockstep, RandomPolicy under AffordableTargetPolicy), in which
// the last damage to a seat is combat damage that also leaves a Wight of
// Precinct Six (toughness 1 + creature cards in its opponents' graveyards)
// with damage between its old and its new toughness. gorge kills it in the
// pass after the loss; the kernel skipped the lost seat's graveyard at once and
// killed it in the loss pass's own batch:
//   - ..._order_...: seed 65, the Wight went to its graveyard before the
//     Death Baron that died beside it (two cards' Pos swapped, decision 1733);
//   - ..._blocker_...: seed 160, a blocking Wight was gone in the kernel while
//     gorge still had it on the battlefield (decision 1676).
func TestFourSeatLostSeatGraveyardCountsUntilSweepLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		name string
		at   int
	}{
		{"lost_seat_graveyard_counts_until_sweep_order_four_seat_pod", 1733},
		{"lost_seat_graveyard_counts_until_sweep_blocker_four_seat_pod", 1676},
	} {
		res := fourSeatPodReproReplay(t, c.name, synthReg(t))
		if res.Div != nil {
			t.Errorf("%s: divergence at %d: %s %s %v", c.name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Decisions <= c.at {
			t.Errorf("%s: %d decisions, the game must go past decision %d", c.name, res.Decisions, c.at)
		}
	}
}
