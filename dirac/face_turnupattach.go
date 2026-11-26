package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The turn-up replacement whose body is an optional attach
// (kop.ReplTurnUpAttach: R:Event$ TurnFaceUp | ValidCard$ Card.Self |
// ReplaceWith$ DB$ Attach | Object$ Self | Optional$ True | Choices$ <spec>;
// Gift of Doom's "As CARDNAME is turned face up, you may attach it to a
// creature"). gorge applyTurnFaceUpReplacements (rules replacement_phase.go)
// parks the TurnFaceUp event for the body's whole continuation: the body's
// ask (gorge effAttach's Min 0 destination KChoose, engine/effects/attach.go)
// is answered while the permanent is STILL FACE DOWN, the attach lands, and
// only then does the fold clear the face-down characteristics and fire the
// event's triggers. With no legal destination the body refuses with a Note
// and the fold proceeds unasked. Cold: a table without the shape never
// reaches these helpers (genPrep.anyTurnUpRepl).

// turnUpAttachOwed: the face-down permanent at sl turns up under a
// ReplTurnUpAttach replacement (true when it has none). A lose-all effect
// silences the face-down permanent's own replacement, as turnUpRepls reads it.
func (s *State) turnUpAttachOwed(k *prep, sl Slot) bool {
	if s.ovrLoses(k, sl) {
		return false
	}
	c := s.cardAt(sl)
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplTurnUpAttach {
			continue
		}
		if a.Repl.Cond != 0 && !s.condEval(k, a.Repl.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
			continue
		}
		return true
	}
	return false
}

// turnUpAttachCands counts (and, with d != nil, offers) the attach's
// destinations: the battlefield permanents the body's Choices$ filter admits
// whose attach the card's Enchant word does not refuse (gorge
// battlefieldValidTargets' pool and attachableBy's rejection), the face-down
// permanent itself never one (attachOne's same-permanent rule).
func (s *State) turnUpAttachCands(k *prep, p uint8, sl Slot, d *Decision) int {
	c := s.cardAt(sl)
	f := kop.FilterID(0)
	for i := range k.t.Cards[c].Abilities {
		a := &k.t.Cards[c].Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplTurnUpAttach {
			f = a.Repl.Filter
			break
		}
	}
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		host := m.first()
		if host == sl || !s.matchPerm(k, f, host, p, sl) || s.auraRefuses(k, sl, host) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(s.perms[host].obj), 0, 0))
		}
	}
	return n
}

// turnUpAttachStage poses the body's attach election before the turn-up
// folds: an empty destination pool refuses the body and the fold proceeds
// (gorge's "cannot attach" Note), a candidate parks the turn-up on the ask.
// The announced X rides pickLeft for the fold's Megamorph counter and
// TurnFaceUp amount.
func (s *State) turnUpAttachStage(k *prep, o Obj, p uint8, mode uint8, x int) bool {
	sl := s.objs[o].slot
	if !s.turnUpAttachOwed(k, sl) || s.turnUpAttachCands(k, p, sl, nil) == 0 {
		return false
	}
	s.g.pickP, s.g.pickZ, s.g.pickF = p, mode, uint16(o)
	s.g.pickLeft = uint8(min(x, 255))
	s.ctl = ctlTurnUpAttach
	return true
}

// turnUpAttachOptions poses the election (gorge's Min 0 ask: an empty answer
// declines too): Done is the decline, each destination a choose:card over it
// (gorge's Optional$ attach election, one pick).
func (s *State) turnUpAttachOptions(k *prep, d *Decision) {
	p, o := s.g.pickP, Obj(s.g.pickF)
	d.Kind, d.Sub, d.Player = DkChoose, ChTurnUpAttach, p
	d.add(MakeAction(AkDone, 0, 0, 0))
	s.turnUpAttachCands(k, p, s.objs[o].slot, d)
}

// applyTurnUpAttach takes the election's answer and folds the turn-up: a
// chosen destination attaches the permanent to it (the legality the ask's
// walk proved, re-read), Done declines, then the face-up characteristics and
// the TurnFaceUp triggers land as gorge's re-emit frame folds the event.
func (s *State) applyTurnUpAttach(k *prep, a Action) {
	p, mode, o, x := s.g.pickP, s.g.pickZ, Obj(s.g.pickF), int(s.g.pickLeft)
	sl := s.objs[o].slot
	if a.Kind() == AkChoose {
		if ob := &s.objs[Obj(a.Source())]; ob.zone == kop.ZoneBattlefield {
			if host := ob.slot; host != sl && !s.auraRefuses(k, sl, host) {
				s.attachTo(k, sl, host)
				s.touchSl(sl)
			}
		}
	}
	s.ctl = ctlPriority
	_, megamorph, _, ok := s.turnUpCost(k, sl, mode)
	if !ok {
		return // the turn-up is stale: the fold's procedure is no longer priced
	}
	s.turnUpDone(k, o, p, mode, x, megamorph)
	s.passes = 0
}

// turnUpAttachEffStage poses the election inside the turning effect's
// resolution (gorge applyTurnFaceUpReplacements parks the event for the
// body's ask): the object rides genState.pickF, the fold waits for the
// answer. It reports whether the ask is parked. Only a card holding the
// replacement poses it (anyTurnUpRepl is also set by the other turn-up
// replacements), and its controller answers it (the body's "you": gorge
// asks the replaced permanent's controller, not the turning effect's).
func (s *State) turnUpAttachEffStage(k *prep, sl Slot) bool {
	p := s.perms[sl].ctrl
	if !s.turnUpAttachOwed(k, sl) || s.turnUpAttachCands(k, p, sl, nil) == 0 {
		return false // no ReplTurnUpAttach on the card, or gorge's body: "cannot attach: no legal target", the fold proceeds
	}
	s.g.pickF = uint16(s.perms[sl].obj)
	s.res.ask, s.res.askP = raGenTurnUpAttach, p
	return true
}

// askOptionsTurnUpAttach builds the election's options: Done is the decline,
// each destination a choose:card over it (gorge's Optional$ attach election);
// false: not this ask.
func (s *State) askOptionsTurnUpAttach(k *prep, d *Decision) bool {
	if s.res.ask != raGenTurnUpAttach {
		return false
	}
	d.Sub = ChTurnUpAttach
	d.add(MakeAction(AkDone, 0, 0, 0))
	s.turnUpAttachCands(k, s.res.askP, s.objs[Obj(s.g.pickF)].slot, d)
	return true
}

// applyAskTurnUpAttach applies the election's answer (Done declines) and
// folds the turn-up the effect began, then the resolution continues.
func (s *State) applyAskTurnUpAttach(k *prep, a Action) bool {
	if s.res.ask != raGenTurnUpAttach {
		return false
	}
	s.res.ask = raNone
	p := s.res.e.ctrl // the turn-up's actor is the turning effect's controller
	o := Obj(s.g.pickF)
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && s.faceDown(k, ob.slot) {
		if a.Kind() == AkChoose {
			if pb := &s.objs[Obj(a.Source())]; pb.zone == kop.ZoneBattlefield {
				if host := pb.slot; host != ob.slot && !s.auraRefuses(k, ob.slot, host) {
					s.attachTo(k, ob.slot, host)
					s.touchSl(ob.slot)
				}
			}
		}
		s.turnUpFlip(k, ob.slot, o, p, 0)
	}
	s.continueRes(k, 0)
	return true
}
