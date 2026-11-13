package dirac

// ExOppsAttacked (gorge Engine.OpponentsAttackedThisTurn): whether this
// turn's declared attackers named a player as their defender. gorge
// derives it from the DeclareAttackers events, one per defender, so a
// declaration aimed only at planeswalkers or battles names no player, and the
// count is complete before any Attacks trigger of the declaration reads it.
// Two players make it 0 or 1.

// noteOppsAttacked records the declaration's player defender among the
// active player's attacking creatures m (finishAttacks, before the
// triggers). A creature chosen again in an extra combat counts again, as
// gorge's event does.
func (s *State) noteOppsAttacked(m slotMask) {
	for ; m.any(); m = m.rest() {
		p := &s.perms[m.first()]
		if p.flags&pfAttacking != 0 && isPlayerOrNone(TCode(p.atk)) {
			s.g.oppAtt = 1
			return
		}
	}
}

// exOppsAttacked is the expression: only the active player attacks, so
// any other controller reads 0.
func (s *State) exOppsAttacked(ctx evalCtx) int {
	if ctx.ctrl != s.active {
		return 0
	}
	return int(s.g.oppAtt)
}
