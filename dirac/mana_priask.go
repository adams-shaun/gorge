package dirac

// A priority-time activation of a colour-asking unless source (kop.Ability.ManaAskColour
// with ManaUnless: Rhystic Cave's "{T}: Choose a color. Add one mana of that color unless
// any player pays {1}", CR 605.1a), under Config.PriorityManaAsk.
//
// gorge's payment windows never list such a source (CR 605.4: an instant-speed-only
// ability, rules untappedManaSource), and its prefix search ends at the other player's
// unless ask, so the colour ask (askColPause) and the unless walk (castWinUnless*) are
// otherwise reachable only inside a cast's window, which gorge never poses for it. The
// knob makes the activation a priority option instead: the {T} is paid at once, the
// activator answers the colour (ChGenManaChoice), every living player is asked in seat
// order whether to pay {1} (ChGenPayMode, the payer's own mana window and tap-another
// pick as in a cast's window), and the mana is added unless one of them paid; priority
// then returns to the activator, no stack object involved (CR 605.3b).
//
// The frame is ctlManaAsk. It has no build, so it parks in the colour window's own
// fields (genState.winSrc, winUnless, winUnlessSrc, winUnlessPay): the colour is not
// asked while winUnlessColor is 0, the unless walk is live once it is set.

// priManaAskAb is the ability index of the unless source at sl that Config.PriorityManaAsk
// offers at priority, or -1.
func (s *State) priManaAskAb(k *prep, sl Slot) int {
	if s.g.priAsk == 0 || k.gen == nil || !s.askColDeferred(k, sl) || !s.askColUnless(k, sl) {
		return -1
	}
	return int(k.gen.small(s.cardAt(sl)).askAb)
}

// priManaAskOffers adds player p's activations of an untapped unless source to the
// priority options (legal.go activateOffers).
func (s *State) priManaAskOffers(k *prep, d *Decision, p uint8) {
	if s.g.priAsk == 0 || k.gen == nil || !k.gen.anyCondSplit {
		return
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if ai := s.priManaAskAb(k, sl); ai >= 0 && s.canTapForMana(k, sl) && s.askColFeePaid(k, p, sl) {
			d.add(MakeAction(AkActivate, TCode(s.perms[sl].obj), 0, uint8(ai)))
		}
	}
}

// priManaAskBegin starts the activation a names when it is an unless source's: the {T} is
// paid and the colour ask is posed next. false: not such an activation.
func (s *State) priManaAskBegin(k *prep, a Action) bool {
	if s.g.priAsk == 0 || a.Kind() != AkActivate || k.gen == nil {
		return false
	}
	sl := s.objs[Obj(a.Source())].slot
	if sl == NoSlot || s.priManaAskAb(k, sl) != int(a.Mode()) {
		return false
	}
	s.windowTapCost(k, sl, false)
	s.g.winSrc, s.g.winUnless, s.g.winUnlessPay = sl, 0, 0
	s.ctl = ctlManaAsk
	return true
}

// priManaAskColour: the frame still waits on the colour answer.
func (s *State) priManaAskColour() bool {
	return s.winUnlessPayer() == 0 && s.winUnlessColor() == 0
}

// priManaAskOptions poses the frame's ask: the activator's colour, then each payer's
// pay/decline (or the payer's tap-another pick).
func (s *State) priManaAskOptions(k *prep, d *Decision) {
	if s.priManaAskColour() {
		d.Kind, d.Player = DkChoose, s.prio
		askColOptions(d)
		return
	}
	s.castWinUnlessOptions(k, d)
}

// applyPriManaAsk answers the frame's ask. The colour answer records the colour and opens
// the unless walk (applyCastWinAsk's unless form); the rest is the colour window's own
// unless answer, which ends in finishManaUnless.
func (s *State) applyPriManaAsk(k *prep, a Action) {
	if !s.priManaAskColour() {
		s.applyCastWinUnless(k, a)
		return
	}
	sl, col := s.g.winSrc, int(a.Mode())
	if !s.askColStart(k, s.prio, sl, col) {
		s.g.winUnlessSrc = sl
		s.winUnlessSetColor(uint8(col))
		s.finishManaUnless(k, true) // the fee went unpaid: the activation makes nothing
		return
	}
	s.g.winUnlessSrc = sl
	s.castWinUnlessStart()
	s.winUnlessSetColor(uint8(col))
	if s.winUnlessPayer() == 0 {
		s.finishManaUnless(k, false) // no living payer: the mana is added
	}
}
