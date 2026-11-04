package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
)

// grantedAttackersBatchFired reports whether a BATCH trig:AttackersDeclared
// line granted under key already fired in this declare step. CR 508.1: the
// declaration is one turn-based action whose DeclareAttackers events are
// grouped per defender, so a granted "whenever you attack" trigger is one
// triggering condition for the whole declaration -- the same (Turn,
// CombatsThisTurn) latch the printed-trigger walk keeps. Per-defender shapes
// (AttackedTarget$) are never latched.
func grantedAttackersBatchFired(e *Engine, t cards.Trigger, key triggerKey) bool {
	if !trigmatch.AttackersDeclaredBatch(t) {
		return false
	}
	return e.attackersDeclaredFired[key] == combatFires{Turn: e.G.Turn, Combat: e.G.CombatsThisTurn}
}

// latchGrantedAttackersBatch commits grantedAttackersBatchFired's latch once
// the granted line has passed every gate, so a rejected match spends nothing.
func latchGrantedAttackersBatch(e *Engine, t cards.Trigger, key triggerKey) {
	if !trigmatch.AttackersDeclaredBatch(t) {
		return
	}
	if e.attackersDeclaredFired == nil {
		e.attackersDeclaredFired = map[triggerKey]combatFires{}
	}
	e.attackersDeclaredFired[key] = combatFires{Turn: e.G.Turn, Combat: e.G.CombatsThisTurn}
}
