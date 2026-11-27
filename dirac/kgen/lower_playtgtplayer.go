package kgen

// playTargetedPlayerArm is the Valid$ Card.TargetedPlayerCtrl arm of
// (*lowerCtx).play's ValidZone$ Hand/Graveyard case: it reports whether the arm
// took the node, and the error it returns is play's. Keeping it out of play
// holds that reader inside its line budget; the shape lives here.
func (lc *lowerCtx) playTargetedPlayerArm(valid, vzone string, forget bool, saExpr string, emit func(lRef, int, string) error) (bool, error) {
	if valid != "Card.TargetedPlayerCtrl" || forget {
		return false, nil
	}
	obj, err := lc.playTargetedPlayerZone(vzone)
	if err != nil {
		return true, err
	}
	zone := zHand
	if vzone == "Graveyard" {
		zone = zGraveyard
	}
	return true, emit(obj, zone, saExpr)
}

// playTargetedPlayerZone resolves the population ref for a Play whose Valid$ is
// Card.TargetedPlayerCtrl over ValidZone$ Hand or Graveyard: the hand or
// graveyard of the player this ability targets (Mindclaw Shaman's revealed
// hand; Sorcerous Squall's milled instant or sorcery). gorge effPlay walks
// every living seat's zone in seat order for that filter
// (playValidReadsOtherHand widens the hand walk; the graveyard is already
// walked over every seat) and keeps the cards the targeted player controls:
// that player's zone, in order. The kernel's CastFree reads the population off
// the player target slot (cast_freetgtplayer.go). The ability must hold exactly
// one target, a player; anything else refuses.
func (lc *lowerCtx) playTargetedPlayerZone(vzone string) (lRef, error) {
	if vzone != "Hand" && vzone != "Graveyard" {
		return lRef{}, refuse("Play Valid$ Card.TargetedPlayerCtrl in ValidZone$ %q", vzone)
	}
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
