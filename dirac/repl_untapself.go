package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A permanent's own "doesn't untap during your untap step" replacement
// (ReplDoesntUntap) with a condition: Alirios, Enraptured's "if you control
// a Reflection" (R:Event$ Untap | ValidCard$ Card.Self |
// ValidStepTurnToController$ You | Layer$ CantHappen | IsPresent$
// Reflection.YouCtrl). gorge reads the IsPresent$ gate when the permanent's
// untap event meets the replacement (rules/replacement_match.go
// replacementConditionHoldsWith, You the replacement's controller), so the
// kernel evaluates Repl.Cond at the same point: false, it untaps.

// UntapCondStats counts the conditional reads (the oracle test's proof the
// pool reaches the gate): Reads every evaluation, Held the ones that kept the
// permanent tapped.
var UntapCondStats struct{ Reads, Held atomic.Int64 }

// selfNoUntap reports whether the permanent at sl's own ReplDoesntUntap keeps
// it tapped in its controller's untap step (the plain one always; a
// conditional one while its Repl.Cond holds).
func (s *State) selfNoUntap(k *prep, sl Slot) bool {
	pc := &k.cards[s.cardAt(sl)]
	if pc.noUntap {
		return true
	}
	return pc.noUntapCond != 0 && s.untapCondHolds(k, sl, pc.noUntapCond)
}

// untapCondHolds evaluates a ReplDoesntUntap's Repl.Cond for the permanent at
// sl, the source and the replacement's controller its own.
func (s *State) untapCondHolds(k *prep, sl Slot, c kop.CondID) bool {
	ok := s.condEval(k, c, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl})
	UntapCondStats.Reads.Add(1)
	if ok {
		UntapCondStats.Held.Add(1)
	}
	return ok
}
