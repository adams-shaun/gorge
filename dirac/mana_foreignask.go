package dirac

// A prefix script's tap that queues a trigger another player controls and
// that poses a choice (targets or a mode) ends the line: gorge's search
// (potential_script.go visit) answers only the payer's own asks, and at any
// other player's non-priority ask it returns false, so no continuation of the
// prefix is offered the play. Relic Bind on a Talisman of Unity: the
// opponent's Taps watcher asks its controller for a mode when the Talisman
// taps, so a play only that tap could fund (Iron Fist's {7}{R} power-up) is
// not offered (CR 603.3c, 605.1a). A foreign trigger that asks nothing (Judge
// of Currents' gain life) is placed and the search goes on.

// foreignPendAsks reports whether a queued trigger controlled by a player
// other than p would pose a choice when placed.
func (s *State) foreignPendAsks(k *prep, p uint8) bool {
	for j := 0; j < int(s.npend); j++ {
		if s.pend[j].ctrl != p && s.pendNeedsDecision(k, j) {
			return true
		}
	}
	return false
}
