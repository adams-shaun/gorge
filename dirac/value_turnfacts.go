package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Read-only turn and timing facts for a search's timing options
// (after work by Daniel Brooks): :waittie's "offered again"
// test (InstantSpeed, NextTurnPlayer, OutlastsTurn) and :raidhold's window
// (HasAttacker). None of them is read unless such an option is on, none
// allocates, and none changes the State.

// HasAttacker reports whether the active player controls a creature that
// could be declared as an attacker now (combat.go anyAttacker: the
// attackers decision's own test), so in the precombat main phase it says
// whether the turn's combat can have an attack (search Options.RaidHold's
// window).
func (s *State) HasAttacker() bool { return s.anyAttacker(s.k()) }

// InstantSpeed reports whether a, an option of the pending priority
// decision, is offered at instant speed: whether the kernel's own timing
// rule would still offer it outside a sorcery-speed window (sorcerySpeed:
// the priority holder is the active player, in a main phase, with an empty
// stack). Every option of a window that is not sorcery-speed is; in a
// sorcery-speed window the options are enumerated again with that
// permission withheld (cfgxNoSorcery) -- the same affordability, target and
// cost checks, only the timing gate shut -- and a is instant-speed when
// that enumeration still offers it. No card names or types are read here:
// the answer is priorityOptions' own. False when the pending decision is
// not a priority one. It sets and clears a transient bit of s, so s must
// not be read concurrently; it allocates nothing.
func (s *State) InstantSpeed(a Action) bool {
	if s.ctl != ctlPriority || s.over {
		return false
	}
	if !s.sorcerySpeed(s.prio) {
		return true
	}
	var d Decision
	s.cfgx |= cfgxNoSorcery
	s.priorityOptions(s.k(), &d)
	s.cfgx &^= cfgxNoSorcery
	return hasOption(&d, a)
}

// NextTurnPlayer is the player whose turn follows the current one, as
// turn.go will begin it: the active player when it has an extra turn
// coming, else the other player.
func (s *State) NextTurnPlayer() uint8 {
	if s.pl[s.active].extraTurns > 0 {
		return s.active
	}
	return s.nextAlive(s.active)
}

// OutlastsTurn reports whether option a of the pending priority decision
// rests on nothing that ends with the turn ("until end of turn" effects
// end in the cleanup step, CR 514.2), so that a later turn's windows could
// offer it again by the timing rules alone. True for exactly two kinds of
// option:
//
//   - a plain or kicked cast (no flashback, face, play-permission, free,
//     warp or other mode bit) of a card in the priority holder's own hand
//     that is instant-speed by its own types or keywords (an instant or a
//     flash card), not by a flash grant;
//   - an activated ability (printed, or granted by an Equipment's static)
//     of a battlefield permanent the priority holder controls other than
//     through an until-end-of-turn control change (pfCtrlEOT), unless its
//     cost has {T} or {Q} and the permanent is a summoning-sick creature
//     with no haste of its own card's (sickTapTurnBound, the 88bf4fa9
//     fix): pfSick clears only at its controller's untap step, so in the
//     opponent's turn the creature is still sick, and haste granted until
//     end of turn (or by another permanent's static) need not last.
//
// Everything else is false. The test is conservative: it may call an
// option that would outlast the turn bound, never the reverse. False when
// the pending decision is not a priority one; it reads s only and
// allocates nothing.
func (s *State) OutlastsTurn(a Action) bool {
	if s.ctl != ctlPriority || s.over {
		return false
	}
	o := Obj(a.Source())
	if int(o) >= int(s.nobj) {
		return false
	}
	ob := &s.objs[o]
	switch a.Kind() {
	case AkCast:
		return a.Mode()&^1 == 0 && ob.zone == kop.ZoneHand && ob.owner() == s.prio && s.k().cards[ob.card].instant
	case AkActivate:
		if ob.zone != kop.ZoneBattlefield {
			return false
		}
		p := &s.perms[ob.slot]
		return p.ctrl == s.prio && p.flags&pfCtrlEOT == 0 && !s.sickTapTurnBound(s.k(), ob.slot, a)
	}
	return false
}

// sickTapTurnBound reports whether activation a of the permanent at sl can
// pay its {T} or {Q} cost only this turn as far as summoning sickness goes
// (CR 302.6): the ability's cost has either symbol and sl is a creature
// with pfSick and no haste of its own card's. a's ability is the card's
// own (mode its index) or, mode bit 0x80, the one an attached permanent
// grants (target: the granting permanent, legal_gen.go); an index past the
// card's abilities reads as turn-bound.
func (s *State) sickTapTurnBound(k *prep, sl Slot, a Action) bool {
	p := &s.perms[sl]
	if p.flags&pfSick == 0 || !s.isCreature(k, sl) {
		return false
	}
	c, ab := s.cardAt(sl), a.Mode()
	if ab&0x80 != 0 {
		host := Obj(a.Target())
		if int(host) >= int(s.nobj) {
			return true
		}
		ab &^= 0x80
		if c = s.objs[host].card; s.objs[host].zone == kop.ZoneBattlefield {
			c = s.grantCardAt(k, s.objs[host].slot, ab)
		}
	}
	abs := k.t.Cards[c].Abilities
	if int(ab) >= len(abs) {
		return true
	}
	if abs[ab].Cost.Flags&(kop.CostTap|kop.CostUntap|kop.CostUntapSelf) == 0 {
		return false
	}
	return !s.ownHaste(k, sl)
}

// ownHaste reports whether sl has haste from its own card's keywords
// (printed, or a copy effect's) and has not lost its abilities (a
// generated table's losses too, lossAt).
func (s *State) ownHaste(k *prep, sl Slot) bool {
	if k.cards[s.cardAt(sl)].kw&kop.KwHaste == 0 || s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return false
	}
	if k.gen != nil {
		if _, lost := s.lossAt(k, sl); lost {
			return false
		}
	}
	return true
}
