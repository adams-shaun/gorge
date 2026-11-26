package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A face-down keyword whose turn-up cost is a single pick part
// (CardDef.MorphPick: Gathan Raiders' Morph—Discard a card, Fathom Seer's
// return two Islands, Gift of Doom's sacrifice another creature; CR 702.37e).
// gorge's keyword turn-up (rules morph_turnup.go) prices the cost's pick part
// beside its mana through morphTurnUpPayable's NonManaCastable, so the action
// is offered only while the controller holds the candidates, and the taken
// action poses the pick ask -- one per object here, gorge's
// turnUpSacAsk / turnUpDiscardAsk / turnUpReturnAsk -- before the payment
// settles and the TurnFaceUp event folds. A manifested or cloaked card's
// printed-mana-cost turn-up is no keyword turn-up and owes no pick (CR
// 701.40b, 701.58b). Cold and out of line: table data and the parked window.
//
// The pick moves as it is answered (the answer's own board change is what
// the next ask's candidate walk reads, gorge's reserved set), so the parked
// window carries only the count still owed (genState.pickLeft) beside the
// player, mode and object (pickP / pickZ / pickF, ctlTurnUpX's shape).

// turnUpPickOwed: the keyword turn-up of the permanent at sl (kw) owes
// CardDef.MorphPick picks.
func (s *State) turnUpPickOwed(k *prep, sl Slot, kw bool) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	return kw && cd.MorphPick.Flags&(kop.CostSac|kop.CostDiscard|kop.CostReturn) != 0
}

// turnUpPickCands counts (and, with d != nil, offers) the pick part's
// candidates for p turning up the face-down permanent at src: a discard part
// the payer's hand cards matching the filter (gorge pay.DiscardCandidates), a
// sacrifice part the permanents p controls matching it that are not
// CantSacrifice-blocked (gorge SacrificeCostCandidates' CostBlocked), a
// return part the permanents p controls matching it (gorge CostCandidates,
// ZBattlefield). The face-down permanent itself is a candidate only when the
// filter names it -- the printed spec is "another ...", which matchPerm's
// source exclusion reads.
func (s *State) turnUpPickCands(k *prep, p uint8, cd *kop.CardDef, src Obj, d *Decision) int {
	pc := &cd.MorphPick
	srcSl := NoSlot
	if src != NoObj && s.objs[src].zone == kop.ZoneBattlefield {
		srcSl = s.objs[src].slot
	}
	n := 0
	switch {
	case pc.Flags&kop.CostDiscard != 0:
		pl := &s.pl[p]
		for i := 0; i < int(pl.handN); i++ {
			o := pl.hand[i]
			if pc.Filter != 0 && !s.matchCardGen(k, pc.Filter, s.objs[o].card, p, p, o) {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	case pc.Flags&kop.CostSac != 0:
		for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
			sl := m.first()
			if sl == srcSl || s.sacBlocked(k, sl) || !s.matchPerm(k, pc.Filter, sl, p, srcSl) {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
			}
		}
	default:
		n = s.returnCands(k, p, pc.Filter, src, d)
	}
	return n
}

// turnUpPickOK: p can pay the keyword turn-up's pick part of the permanent at
// sl (true when it owes none): the candidates still to take cover the picks
// still owed. The picks made inside the parked window moved already, so the
// walk reads the board the next ask sees.
func (s *State) turnUpPickOK(k *prep, p uint8, sl Slot) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	if !s.turnUpPickOwed(k, sl, true) {
		return true
	}
	owed := int(cd.MorphPick.N)
	made := 0
	if s.ctl == ctlTurnUpPick && s.g.pickF == uint16(s.perms[sl].obj) {
		made = owed - int(s.g.pickLeft)
	}
	return s.turnUpPickCands(k, p, cd, s.perms[sl].obj, nil)+made >= owed
}

// turnUpPickStage runs the turn-up's pick part before it settles (gorge
// advanceTurnUp's asks, ahead of the payment). gorge records a singleton
// pick without an ask (turnUpSacAsk / turnUpDiscardAsk / turnUpReturnAsk's
// no-choice shortcut) and poses the rest as one exact-N KChoose, one pick per
// repose here. It reports whether the turn-up is handled here: parked on its
// ask, or the picks taken unasked.
func (s *State) turnUpPickStage(k *prep, o Obj, p uint8, mode uint8, kw bool) bool {
	sl := s.objs[o].slot
	if !s.turnUpPickOwed(k, sl, kw) {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	left := int(cd.MorphPick.N)
	for left > 0 {
		n := s.turnUpPickCands(k, p, cd, o, nil)
		if left == 1 && n == 1 {
			// gorge's no-choice shortcut: recorded without an ask, and the
			// settlement follows within the same action here
			var d Decision
			s.turnUpPickCands(k, p, cd, o, &d)
			s.turnUpPickMove(k, &cd.MorphPick, Obj(d.Options[0].Source()))
			left--
			continue
		}
		s.g.pickP, s.g.pickZ, s.g.pickF = p, mode, uint16(o)
		s.g.pickLeft = uint8(left)
		s.ctl = ctlTurnUpPick
		return true
	}
	// every pick taken unasked: the settle reads them made through the window.
	// A stale settle (it pays nothing and turns nothing up) would leave the
	// picks paid and the card face down; the lowering admits a pick only with
	// a {0} MorphCost and no X, life or reveal part (kgen lower_morphpick.go),
	// so nothing beside the picks can fail the settle. A mana part allowed
	// beside a pick must re-check the settle before the picks move.
	s.g.pickP, s.g.pickZ, s.g.pickF = p, mode, uint16(o)
	s.g.pickLeft = 0
	s.ctl = ctlTurnUpPick
	if s.settleTurnUp(k, o, p, mode, 0) {
		return true // face_turnupattach.go: the attach ask parks the turn-up still face down
	}
	s.ctl = ctlPriority
	return true
}

// turnUpPickOptions poses the next pick ask of a parked turn-up.
func (s *State) turnUpPickOptions(k *prep, d *Decision) {
	p, o := s.g.pickP, Obj(s.g.pickF)
	sl := s.objs[o].slot
	pc := &k.t.Cards[s.cardAt(sl)].MorphPick
	d.Kind, d.Player = DkChoose, p
	switch {
	case pc.Flags&kop.CostDiscard != 0:
		d.Sub = ChCostDiscard
	case pc.Flags&kop.CostSac != 0:
		d.Sub = ChCostSacrifice
	default:
		d.Sub = ChCostReturn
	}
	d.N = 0
	s.turnUpPickCands(k, p, &k.t.Cards[s.cardAt(sl)], o, d)
}

// applyTurnUpPick takes the answered pick of a parked turn-up and moves it as
// gorge's settlement pays the part (the discard to the graveyard, the return
// to its owner's hand, the sacrifice after the sac trigger): more picks keep
// the window parked, the last one settles the turn-up.
func (s *State) applyTurnUpPick(k *prep, a Action) {
	p, mode, o := s.g.pickP, s.g.pickZ, Obj(s.g.pickF)
	sl := s.objs[o].slot
	s.turnUpPickMove(k, &k.t.Cards[s.cardAt(sl)].MorphPick, Obj(a.Source()))
	s.g.pickLeft--
	if s.g.pickLeft > 0 {
		return // the next ask is posed from the parked window
	}
	if s.settleTurnUp(k, o, p, mode, 0) {
		return // face_turnupattach.go: the attach ask parks the turn-up still face down
	}
	s.ctl = ctlPriority
}

// turnUpPickMove moves the picked object as gorge's settlement pays the part
// (the discard to the graveyard, the return to its owner's hand, the
// sacrifice after the sac trigger).
func (s *State) turnUpPickMove(k *prep, pc *kop.Cost, o Obj) {
	switch {
	case pc.Flags&kop.CostDiscard != 0:
		s.discardObj(k, o)
	case pc.Flags&kop.CostSac != 0:
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
			s.fireSac(k, ob.slot)
			s.leave(k, ob.slot, kop.ZoneGraveyard, false)
		}
	default:
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
			s.leave(k, ob.slot, kop.ZoneHand, false)
		}
	}
}

// turnUpAskOptions poses the turn-up window parked in s.ctl: the X pick
// (face_turnfaceup.go), the reveal pick (act_revealpick.go), the cost pick or
// the attach election. One dispatch so stepRaw holds one case for the four.
func (s *State) turnUpAskOptions(k *prep, d *Decision) {
	switch s.ctl {
	case ctlTurnUpX:
		s.turnUpXOptions(d)
	case ctlTurnUpReveal:
		s.turnUpRevealOptions(k, d)
	case ctlTurnUpPick:
		s.turnUpPickOptions(k, d)
	case ctlTurnUpAttach:
		s.turnUpAttachOptions(k, d)
	}
}
