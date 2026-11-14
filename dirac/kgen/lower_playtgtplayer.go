package kgen

// playTargetedPlayerHand lowers a Play whose Valid$ is Card.TargetedPlayerCtrl
// over ValidZone$ Hand: the hand of the player this ability targets
// (Mindclaw Shaman's revealed hand). gorge effPlay walks every living seat's
// hand in seat order for that filter (playValidReadsOtherHand widens the
// walk) and keeps the cards the targeted player controls: that player's
// hand, in order. The kernel's CastFree reads the population off the player
// target slot (cast_freetgtplayer.go). The ability must hold exactly one
// target, a player; anything else refuses. The graveyard spelling
// (Sorcerous Squall) stays refused: no lockstep game has cast it yet.
func (lc *lowerCtx) playTargetedPlayerHand() (lRef, error) {
	slot, nTgts := -1, 0
	for i, t := range lc.ab.Targets {
		if lc.mode >= 0 && t.Mode != lc.mode {
			continue
		}
		nTgts++
		if t.F != nil && t.F.Players != 0 && len(t.F.Clauses) == 0 {
			slot = i
		}
	}
	if slot < 0 || nTgts != 1 {
		return lRef{}, refuse("Play Valid$ Card.TargetedPlayerCtrl without one player target")
	}
	return lRef{Kind: rTarget, N: slot}, nil
}
