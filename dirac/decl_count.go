package dirac

// decl_count.go: which answers State.Decisions counts. gorge answers one of
// its multi-selects with one intent, and its search seed folds the
// decisions answered before the pending one (decision.Decision.Ordinal); the
// kernel poses that multi-select as single picks (doc.go "Decision
// grammar"). So only the pick that closes a declaration is counted: SeqNo is
// gorge's Ordinal at every pick of it.

// pickKeepsOpen reports whether answering the pending pick with a leaves the
// same gorge decision open. Apply calls it before it answers.
func (s *State) pickKeepsOpen(k *prep, a Action) bool {
	switch s.ctl {
	case ctlAttackers, ctlBlockers:
		return a.Kind() != AkDone // gorge KAttackers / KBlockers, closed by AkDone
	case ctlPlaceTriggers:
		return s.orderLeft > 1 // gorge KTriggerOrder, Min=Max=n
	case ctlCleanupDiscard:
		return s.discardLeft > 1 // gorge KChoose "discard", Min=Max=excess
	case ctlMullBottom:
		return s.mullLeft > 1 // gorge KMulligan bottom, one pick per card
	case ctlBuild:
		return s.buildPickKeepsOpen(k, a)
	case ctlResolve:
		return s.resolvePickKeepsOpen(a)
	}
	return false
}

// buildPickKeepsOpen: the picks of a build (applyBuild's dispatch order).
func (s *State) buildPickKeepsOpen(k *prep, a Action) bool {
	if s.winUnlessPayer() != 0 || s.bld().bfl&bflWinTap != 0 {
		return false
	}
	b := s.bld()
	if k.gen != nil && s.g.mt.active && b.stage == bsTargets {
		// a multi-object target slot (gorge KTarget Min/Max): AkDone, or the
		// pick that reaches the maximum, closes it (applyMultiTarget)
		return s.g.mt.slot != mtCharm && a.Kind() == AkTarget && s.g.mt.picked+1 < s.g.mt.max
	}
	if a.Kind() != AkChoose {
		return false
	}
	switch b.stage {
	case bsDivide:
		return b.divLeft > 1 // gorge's CR 601.2d "divide" ask, one pick per point
	case bsTapCost, bsCosts:
		// gorge KChoose "tapcost" / "sacrifice" / "discard", Min=Max=N
		switch b.pickK {
		case pkTapX, pkSac, pkDiscard:
			return b.pickLeft > 1
		}
	}
	return false
}

// resolvePickKeepsOpen: the picks of a resolution ask (applyResolve).
func (s *State) resolvePickKeepsOpen(a Action) bool {
	r := &s.res
	switch r.ask {
	case raDiscard: // gorge KModes "discard", one pick per card
		return r.left > 1 && int(s.pl[r.chosen].handN) > int(r.peekI)+1
	case raScryTop, raSurveil: // gorge KArrange: the top picks, then pile B's order
		if r.flags&rfRestPh != 0 {
			return int(r.peekN)-int(r.peekI)-1 > 1
		}
		if a.Kind() == AkChoose {
			return r.peekI+1 < r.peekN
		}
		return s.cfgx&cfgxArrangeRest != 0 && int(r.peekN)-int(r.peekI) >= 2
	}
	return false
}
