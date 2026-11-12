package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// two controller roles of a battlefield filter and the
// stack read of a target-gated cost reduction. Cold and out of line:
// flags2Perm reaches ctrlRoleF2 only through one combined bit test, and
// costTargetMatch runs only for a StTargetsFilter2 static, so a table
// without these filters pays one AND per Flags2-carrying filter evaluation

// ctrlRoleF2 reports whether a permanent controlled by ctrl passes the
// controller-role bits of f.
//
//   - F2TgtPlayerCtrl (gorge matchTargetedPlayerCtrl, CR 115.10): ctrl is a
//     player target of the current stack entry (the resolving one, else the
//     one being built). No entry, or no player target, is false: gorge
//     leaves the referent unbound rather than guessed.
//   - F2DefendingCtrl (gorge controlReferentPlayers TriggeredDefendingPlayer,
//     the Attacks trigger's DefendingPlayer, CR 508.5): two-player, only the
//     active player attacks (CR 506.2) and its attackers attack the other
//     seat or a permanent that seat controls or protects, so the defending
//     player is the non-active one. kgen lowers it only in an Attacks
//     trigger's body. Past two seats it is the seat the trigger's attacker
//     attacks (defendingSeatF2).
func (s *State) ctrlRoleF2(k *prep, f kop.FilterFlags2, ctrl uint8) bool {
	if f&kop.F2DefendingCtrl != 0 {
		if MaxSeats == 2 {
			if ctrl == s.active {
				return false
			}
		} else if ctrl != s.defendingSeatF2(k) {
			return false
		}
	}
	if f&kop.F2TgtPlayerCtrl != 0 {
		if DebugTgtPlayerCtrlAny {
			return true // the test-only mutation
		}
		e := s.curEntry()
		if e == nil {
			return false
		}
		// the slots and a multi-pick slot's extras (Officious Interrogation's
		// second player)
		var codes [kop.MaxTargets + maxXTgt]uint8
		for _, c := range codes[:s.chainTargets(e, &codes)] {
			if c == PlayerCode(ctrl) {
				return true
			}
		}
		return false
	}
	return true
}

// ctrlRoleTgt reports whether a permanent controlled by ctrl passes a
// Filter.CtrlTgt role: the controller is the controller of target slot
// slot (index), where a player target is that player, a stack ability or a
// spell on the stack its controller, and an object target the controller of
// its permanent or, once it has left, its owner (gorge
// controlReferentPlayers TargetedOrController). The resolving entry reads
// the raw slot: an earlier target this resolution already judged illegal
// still names its object, so it cannot turn a later link's filter false.
// The entry being built has the slots chosen so far. No entry, no such slot
// or no target is false: gorge leaves the referent unbound.
func (s *State) ctrlRoleTgt(k *prep, slot int, ctrl uint8) bool {
	e := s.curEntry()
	if e == nil || slot < 0 || slot >= int(e.nt) {
		return false
	}
	// the raw slot, not tgtCode: the illegal mark hides the referent from
	// the later link's target recheck, which gorge does not do
	t := e.tgt[slot]
	switch {
	case t == NoTarget:
		return false
	case isPlayerTarget(t):
		return ctrl == CodeSeat(t)
	case isStackTarget(t):
		if s.res.active {
			return ctrl == s.stackTgtCtrl(slot)
		}
		if j := s.stackTgtIdx(t, e.tver[slot]); j >= 0 {
			return ctrl == s.stack[j].ctrl
		}
		return false
	case !isObjTarget(t):
		return false // 0xFE, a departed stack target, names no controller
	case s.objs[t].zone == kop.ZoneStack:
		if j := s.stackIndexOf(Obj(t)); j >= 0 {
			return ctrl == s.stack[j].ctrl // a spell is controlled by its caster (CR 108.4)
		}
	}
	return ctrl == s.ctrlOfObj(Obj(t))
}

// defendingSeatF2 is the defending player of an Attacks trigger past two
// seats (gorge TriggeredDefendingPlayer, CR 508.5), bound when the trigger is
// queued: while the trigger fires (an IsPresent$ condition) it is the
// checked event's seat (g.atkDef), afterwards the current entry's defSeat.
// A read with neither stalls the game (StallDefenderPW, a declared limit)
// and answers NoTarget, which matches no seat.
func (s *State) defendingSeatF2(k *prep) uint8 {
	if DebugDefendingActiveOpp {
		return (s.active + 1) % s.nSeats() // seat-ok: a test-only mutation (DebugDefendingActiveOpp)
	}
	if s.g.atkDef != 0 {
		return s.g.atkDef - 1
	}
	if e := s.curEntry(); e != nil && e.kind == seTrigger && e.defSeat != 0 {
		return e.defSeat - 1
	}
	s.capFull(StallDefenderPW)
	return NoTarget
}

// DebugDefendingActiveOpp is a test-only mutation: past two seats the
// defending player of an Attacks trigger is read as the seat after the
// active one, the two-seat reduction, whoever the attacker attacks.
var DebugDefendingActiveOpp bool

// costTargetMatch reports whether target code t matches the Filter2 f of a
// target-gated cost reduction (gorge costTargetsMatch, CR 601.2f): a
// battlefield permanent, or for a stack-zone f (a Spell head, Out of Air's
// Spell.Creature) a spell on the stack, read like a TgtSpell target
// (legal.go targetOK). Every Filter2 reader (costTargetOK's build and probe
// branches, affordableTargets) goes through it.
func (s *State) costTargetMatch(k *prep, f kop.FilterID, p uint8, t uint8) bool {
	return s.costTargetMatchSrc(k, f, p, t, NoSlot)
}

// costTargetMatchSrc is costTargetMatch for a filter read against the cost
// static's source permanent src (a Card.Self ValidTarget$: Terror of the Peaks).
func (s *State) costTargetMatchSrc(k *prep, f kop.FilterID, p uint8, t uint8, src Slot) bool {
	if isPlayerOrNone(t) {
		return false
	}
	ob := &s.objs[t]
	if k.t.Filters[f].Zone == kop.ZoneStack {
		return ob.zone == kop.ZoneStack && s.matchCardAt(k, f, ob.card, s.stackCtrl(Obj(t)), p, Obj(t))
	}
	return ob.zone == kop.ZoneBattlefield && s.matchPermGen(k, f, ob.slot, p, src, layerFull)
}
