package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ChangeCombatants (CR 506.3b / 508.1; gorge engine/effects/changecombatants.go
// effChangeCombatants's Attacking$ True arm): the resolving controller
// reselects which player each defined attacking creature attacks. The kernel
// reads the effect's subject like gorge's Defined walk: one target object
// (ValidTgts$) or every battlefield permanent matching a Defined$ Valid
// filter, in slot order. For each subject that is a creature attacking in its
// controller's own combat (the active player's), the candidates are every
// living seat except the attacker's controller in gorge's AliveFrom(0) order.
// One ask per attacker (raCombatant, ChGenPlayer options); a pick that MOVES
// the attack sets the new defending seat and clears the attacker's blockers,
// exactly the fold of gorge's CombatRetarget (events foldCombatRetarget: the
// attacker becomes unblocked, a former blocker that still blocks another
// attacker keeps IsBlocking). A pick of the current defender is a true no-op
// (gorge emits no CombatRetarget), and an attacker whose only candidate is the
// defender it already attacks asks nothing.
//
// DebugChangeCombatantsKeepBlockers keeps a moved attack's old block list
// (the retarget path's state-level break). DebugChangeCombatantsNoMove is the
// lockstep mutation; see its own comment.

// DebugChangeCombatantsKeepBlockers makes a moved attack keep its blockers.
var DebugChangeCombatantsKeepBlockers bool

// DebugChangeCombatantsNoMove is the lockstep mutation: the ask is answered
// but the attack stays on its current defender, so a game that redirects must
// diverge at the next combat decision. DebugChangeCombatantsKeepBlockers
// cannot be the lockstep mutation for these carriers: their trigger is during
// the declare attackers step, before any blocker exists, so there is nothing
// for it to keep.
var DebugChangeCombatantsNoMove bool

// changeCombatantsExt is ExtChangeCombatants.
//
//kmanifest:ext ExtChangeCombatants
func (s *State) changeCombatantsExt(k *prep, in *kop.Instr) (ok, done bool) {
	if s.changeCombatantNext(k, in, 0) {
		return true, true
	}
	return false, true
}

// changeCombatantNext asks for the first redirectable attacker of in at or
// after slot from; true when nothing is left to ask.
func (s *State) changeCombatantNext(k *prep, in *kop.Instr, from Slot) bool {
	if in.Obj != kop.RefNone {
		sl := s.objRef(k, in.Obj)
		if sl == NoSlot || sl < from {
			return true
		}
		return !s.changeCombatantPose(k, sl)
	}
	if in.F == 0 {
		return true
	}
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	for i := 0; i < n; i++ {
		sl := ord[i]
		if sl < from {
			continue
		}
		if s.changeCombatantPose(k, sl) {
			return false
		}
	}
	return true
}

// changeCombatantPose asks the resolving controller for attacker sl's new
// defender; false when sl is not a redirectable attacker (gorge skips it).
func (s *State) changeCombatantPose(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	if p.flags&pfAttacking == 0 || p.ctrl != s.active || s.objs[p.obj].zone != kop.ZoneBattlefield {
		return false
	}
	cur, hasCur := s.attackDefendingSeat(k, sl)
	var cands [MaxSeats]uint8
	n := 0
	q := s.aliveFrom(0)
	for i := 0; i < int(q.n); i++ {
		if q.s[i] != p.ctrl {
			cands[n] = q.s[i]
			n++
		}
	}
	if n == 0 || n == 1 && hasCur && cands[0] == cur {
		// gorge: no Note-able candidate / the one legal answer keeps the
		// attack where it is, so no decision anybody could answer
		// differently is emitted
		return false
	}
	s.res.chPl = uint8(sl) + 1
	s.res.ask, s.res.askP = raCombatant, s.res.e.ctrl
	return true
}

// attackDefendingSeat is the seat the attacker at sl is attacking: a player
// target itself, or an attacked permanent's controller. hasDef is false when
// the attack names no live defender.
func (s *State) attackDefendingSeat(k *prep, sl Slot) (seat uint8, hasDef bool) {
	t := s.perms[sl].atk
	if isPlayerTarget(t) {
		return CodeSeat(t), true
	}
	if isObjTarget(t) {
		if q := s.attackedPerm(k, sl); q != NoSlot {
			return s.perms[q].ctrl, true
		}
	}
	return 0, false
}

// changeCombatantOptions poses attacker res.chPl-1's defender pick: one
// "player" option per living seat except its controller, in AliveFrom(0)
// order. The answer's Source names the attacker, its Mode the seat.
//
//kmanifest:askopts raCombatant
func (s *State) changeCombatantOptions(k *prep, d *Decision) {
	sl := Slot(s.res.chPl - 1)
	d.Sub = ChGenPlayer
	ctrl := s.perms[sl].ctrl
	q := s.aliveFrom(0)
	for i := 0; i < int(q.n); i++ {
		if q.s[i] != ctrl {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, q.s[i]))
		}
	}
}

// changeCombatantAnswer applies the pick and moves to the next attacker.
//
//kmanifest:askapply raCombatant
func (s *State) changeCombatantAnswer(k *prep, a Action) {
	cursor := s.res.chPl
	s.res.ask = raNone
	s.res.chPl = 0
	if cursor == 0 {
		s.continueRes(k, 0)
		return
	}
	sl := Slot(cursor - 1)
	seat := a.Mode()
	p := &s.perms[sl]
	cur, hasCur := s.attackDefendingSeat(k, sl)
	if seat < s.nSeats() && !s.out(seat) && seat != p.ctrl && (!hasCur || seat != cur) && !DebugChangeCombatantsNoMove {
		s.changeCombatantMove(k, sl, seat)
	}
	if s.changeCombatantNext(k, &s.prog(k)[s.res.pc], sl+1) {
		s.continueRes(k, 0)
	}
}

// changeCombatantMove moves attacker sl's attack to seat and clears its
// blockers, the fold of gorge's CombatRetarget (DebugChangeCombatantsKeepBlockers
// keeps the old block list).
func (s *State) changeCombatantMove(k *prep, sl Slot, seat uint8) {
	p := &s.perms[sl]
	if !DebugChangeCombatantsKeepBlockers {
		for bm := s.oppsM(s.active); bm.any(); bm = bm.rest() {
			b := bm.first()
			if !s.blocks(b, sl) {
				continue
			}
			s.forgetBlocked(b, sl)
			if s.perms[b].blocking == NoSlot {
				s.perms[b].flags &^= pfBlocking
			}
		}
		p.nblock = 0
		p.flags &^= pfBlocked
	}
	s.setAttackTarget(p, PlayerCode(seat))
	s.touchSl(sl)
}
