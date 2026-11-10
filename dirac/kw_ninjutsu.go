package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ninjutsu (CR 702.49), cold: reached only when the table
// has a K:Ninjutsu ability (genPrep.anyNinjutsu; kop.AfNinjutsu).
//
// gorge expands the keyword into one hand ability (engine/cards/
// kw_ninjutsu.go): "{cost}, Return an unblocked attacker you control to
// hand: Put this card onto the battlefield from your hand tapped and
// attacking", ActivationPhases$ Declare Blockers,Combat Damage,EndCombat.
// The kernel offers it through the ordinary hand-ability walk
// (priorityHandAct) inside that window (gorge activationPhasesOK; gorge's
// single StepCombatDamage is the kernel's first-strike and regular damage
// steps) and only while a Return candidate exists (costObjPayable). The
// Return<1/Creature.YouCtrl+attacking+unblocked> part is the activation
// Return machinery (act_energy.go pkReturn); as its pick is made, while the
// creature still attacks, the entry's trigObj captures that creature's
// defender (perm.atk: a PlayerTarget* or a planeswalker Obj; gorge
// cast_payment.go castAnswerReturncost -> foldNinjutsuDefender). An
// activated ability has no trigger object of its own (activateGen sets
// NoObj for a hand source), and its program is OpNinjutsu alone.
//
// OpNinjutsu (gorge effChangeZone's object path, zone_change.go): when the
// card is still in its owner's hand it enters the battlefield (MoveZone:
// ETB triggers), is tapped (the "entered tapped" Tap: Taps triggers), then
// enters attacking the captured defender when that is still legal
// (attackingEntry.apply: EnteringAttackLegal, TokenAttacks). It was never
// declared as an attacker: no Attacks trigger, no attacked count (CR
// 508.4), and no blocker can block it.

// DebugNinjutsuAnyStep offers ninjutsu outside its window too (mutation
// testing only: the lockstep must catch it).
var DebugNinjutsuAnyStep bool

// DebugNinjutsuNoAttack makes the ninja enter tapped but not attacking
// (mutation testing only).
var DebugNinjutsuNoAttack bool

// ninjutsuStep reports whether the current step is the ninjutsu window.
func (s *State) ninjutsuStep() bool {
	switch s.step {
	case kop.StepBlockers, kop.StepFirstStrikeDamage, kop.StepDamage, kop.StepEndCombat:
		return true
	}
	return DebugNinjutsuAnyStep
}

// ninjutsuOfferOK gates the hand offer of ninjutsu ability a of card o:
// the window, and a Return candidate (gorge's cost-object census).
func (s *State) ninjutsuOfferOK(k *prep, a *kop.Ability, p uint8, o Obj) bool {
	return s.ninjutsuStep() && s.costObjPayable(k, p, &a.Cost, o, NoObj)
}

// ninjutsuEntry reports whether e (an entry being built) is a ninjutsu
// activation.
func ninjutsuEntry(k *prep, e *stackEntry) bool {
	if e.kind != seAbility || int(e.ab) >= len(k.t.Cards[e.card].Abilities) {
		return false
	}
	return k.t.Cards[e.card].Abilities[e.ab].AFlags&kop.AfNinjutsu != 0
}

// ninjutsuCapture records the returned creature's defender on the entry
// being built as the Return pick of object o is made.
func (s *State) ninjutsuCapture(o Obj) {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		s.bld().e.trigObj = Obj(s.perms[ob.slot].atk)
		s.syncBuild()
	}
}

// ninjutsuResolve is OpNinjutsu: the resolving entry's source card, if
// still in its owner's hand, enters tapped and attacking.
//
//go:noinline
func (s *State) ninjutsuResolve(k *prep) {
	e := &s.res.e
	o := e.obj
	ob := &s.objs[o]
	if ob.zone != kop.ZoneHand || ob.ver != e.srcVer {
		// gorge changeZoneMoveSkip: no longer in its Origin$ zone; and
		// sourceReturnedSincePush: a card that entered the battlefield and
		// came back to the hand is a new object (CR 400.7; enterNF bumps
		// ver)
		return
	}
	if s.occ.full() {
		s.stall(StallBoard) // the battlefield is full: the capacity draw
		return
	}
	ctrl := ob.owner()
	from := s.detach(o)
	sl := s.enter(k, o, ctrl, false, from)
	pp := &s.perms[sl]
	if pp.flags&pfTapped == 0 {
		pp.flags |= pfTapped
		s.touchSl(sl)
		if s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, sl)
		}
		s.markTapped(k, sl)
	}
	if DebugNinjutsuNoAttack || !s.enteringAttackLegal(k, ctrl, e.trigObj) {
		return
	}
	pp.flags |= pfAttacking
	pp.atk = uint8(e.trigObj)
	pp.bord = s.nAttackers
	s.nAttackers++
	s.touchSl(sl)
}
