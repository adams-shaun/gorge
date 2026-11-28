package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Count$CardNumAttacksThisTurn (kop.ExCardNumAttacksThisTurn; Moraug, Fury of
// Akoum's "+1/+0 for each time it has attacked this turn"): how many times
// the reading object has been declared as an attacker this turn (gorge
// state.Object.AttacksThisTurn, which events.Apply's DeclareAttackers fold
// increments and TurnChange resets; extra combats share the turn, so a
// creature that attacks in two combats reads 2). The kernel keeps the tally
// per object in the cold region coldAtkN, only for a table some expression
// of which reads it (genPrep.anyAtkTally). finishAttacks counts the declared
// attackers before the declaration's triggers fire (gorge matches triggers
// on the folded event, the attack already counted); the turn start clears it
// with the attacked marks (turnTalliesNewTurn). The read is ctx.src's own
// count, so the AffectedX static (kop.StPerAffected) reads each affected
// creature's (chars_gen.go re-anchors ctx.src) and a plain SVar the source's.

var coldAtkN = newCold[uint8]("atkN", func(g *genPrep) int {
	if g.anyAtkTally {
		return MaxObj
	}
	return 0
})

// DebugAtkTallyOnce (TestHostedCmdrMoraugAttackTallyLockstep's mutation): a
// declaration counts only an object's first attack of the turn.
var DebugAtkTallyOnce bool

// prepAtkTally reads whether some expression of t is ExCardNumAttacksThisTurn.
func prepAtkTally(t *kop.Table, g *genPrep) {
	for i := range t.Exprs {
		if t.Exprs[i].Kind == kop.ExCardNumAttacksThisTurn {
			g.anyAtkTally = true
			return
		}
	}
}

// tallyAttacks counts one attack for every attacker the active player just
// declared (finishAttacks).
func (s *State) tallyAttacks() {
	n := coldAtkN.of(s)
	if n == nil {
		return
	}
	for m := s.ctrlM[s.active]; m.any(); m = m.rest() {
		if sl := m.first(); s.perms[sl].flags&pfAttacking != 0 {
			if o := s.perms[sl].obj; int(o) < len(n) && n[o] < 255 && !(DebugAtkTallyOnce && n[o] > 0) {
				n[o]++
			}
		}
	}
}

// atkTallyNewTurn clears the tallies as the turn begins.
func (s *State) atkTallyNewTurn() {
	clear(coldAtkN.of(s))
}

// atkTally is ExCardNumAttacksThisTurn: ctx.src's attacks this turn (0 for
// no source, as gorge's missing-source read).
func (s *State) atkTally(ctx evalCtx) int {
	if n := coldAtkN.of(s); int(ctx.src) < len(n) {
		return int(n[ctx.src])
	}
	return 0
}
