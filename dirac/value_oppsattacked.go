package dirac

import "math/bits"

// ExOppsAttacked (gorge Engine.OpponentsAttackedThisTurn): the distinct
// players this turn's declared attackers named as their defender. gorge
// derives it from the DeclareAttackers events, one per defender, so a
// declaration aimed only at planeswalkers or battles names no player, and the
// count is complete before any Attacks trigger of the declaration reads it.
//
// Expr.N 1 is melee's count (cards.MeleePumpCount, gorge meleeRemembered):
// the distinct defending seats of the declaration that queued the trigger, a
// planeswalker's or battle's attack counting its controller's or protector's
// seat (CR 702.121b). The trigger resolves before any later declaration, so
// the declaration's mask is its capture.
//
// genState.oppAtt keeps both: bits 0-3 the seats named as player defenders
// this turn, bits 4-7 the latest declaration's defending seats.

// DebugMeleeOne is the mutation switch: melee counts one opponent however
// many the declaration attacked, the old two-seat reading. Tests only.
var DebugMeleeOne bool

// DebugOppsAttOne is the mutation switch: the opponents attacked this turn
// read as at most one (The Beacon of Kahless's old two-seat reading). Tests
// only.
var DebugOppsAttOne bool

// noteOppsAttacked records the declaration's defenders among the active
// player's attacking creatures m (finishAttacks, before the triggers). A
// creature chosen again in an extra combat counts again, as gorge's event
// does.
func (s *State) noteOppsAttacked(k *prep, m slotMask) {
	turn, decl := s.g.oppAtt&0x0f, uint8(0)
	for ; m.any(); m = m.rest() {
		a := m.first()
		p := &s.perms[a]
		if p.flags&pfAttacking == 0 {
			continue
		}
		d := s.atkSeat(k, a)
		if d >= MaxSeats {
			continue
		}
		decl |= 1 << d
		if isPlayerOrNone(p.atk) {
			turn |= 1 << d
		}
	}
	s.g.oppAtt = turn | decl<<4
}

// exOppsAttacked is the expression: only the active player attacks, so
// any other controller reads 0.
func (s *State) exOppsAttacked(x int16, ctx evalCtx) int {
	if ctx.ctrl != s.active {
		return 0
	}
	if x == 1 {
		if DebugMeleeOne {
			return min(1, bits.OnesCount8(s.g.oppAtt>>4))
		}
		return bits.OnesCount8(s.g.oppAtt >> 4)
	}
	if DebugOppsAttOne {
		return min(1, bits.OnesCount8(s.g.oppAtt&0x0f))
	}
	return bits.OnesCount8(s.g.oppAtt & 0x0f)
}
