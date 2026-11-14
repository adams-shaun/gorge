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

// pcard.noUntap values.
const (
	untapPlain = 1 // an unconditional ReplDoesntUntap
	untapGated = 2 // only conditional ones: Repl.Cond is read at the untap
)

// noUntapKind folds one ReplDoesntUntap (its Repl.Cond) into the card's
// pcard.noUntap: a plain one wins over any number of conditional ones.
func noUntapKind(cur uint8, cond kop.CondID) uint8 {
	if cond == 0 {
		return untapPlain
	}
	if cur == 0 {
		return untapGated
	}
	return cur
}

// selfNoUntap reports whether the permanent at sl's own ReplDoesntUntap keeps
// it tapped in its controller's untap step (the plain one always; a
// conditional one while its Repl.Cond holds). The pcard carries only the
// two-valued flag (a wider pcard costs the whole kernel instructions).
func (s *State) selfNoUntap(k *prep, sl Slot) bool {
	pc := &k.cards[s.cardAt(sl)]
	if k.gen != nil && s.genLost(k, sl) {
		return false // a lost "doesn't untap" (Witness Protection)
	}
	if pc.noUntap == untapPlain {
		return true
	}
	if pc.noUntap != untapGated {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDoesntUntap && a.Repl.Cond != 0 && s.untapCondHolds(k, sl, a.Repl.Cond) {
			return true
		}
	}
	return false
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
