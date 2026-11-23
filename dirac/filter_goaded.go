package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugCountGoadedHits switches DebugGoadedPredicateHits on. It is off in
// production, so the predicate's hot path is one shared read and no shared
// write (parallel kernel workers must not contend on a counter).
var DebugCountGoadedHits atomic.Bool

// DebugGoadedPredicateHits counts IsGoaded predicate matches while
// DebugCountGoadedHits is set; oracle tests read it to prove a lockstep
// actually evaluated the word (a green game in which no goad exists would
// prove nothing).
var DebugGoadedPredicateHits atomic.Int64

// goadedPredicate is PredGoaded: the battlefield permanent at sl is goaded by
// any player (gorge IsGoaded, engine/effects/filter.go matchPositive), with
// PredN 1 the positive word and 0 its ! negation. goadedAt is the same read
// the combat requirement uses, so the two routes cannot disagree.
func (s *State) goadedPredicate(k *prep, fl *kop.Filter, sl Slot) bool {
	if s.goadedAt(k, sl) == (fl.PredN == 1) {
		if DebugCountGoadedHits.Load() {
			DebugGoadedPredicateHits.Add(1)
		}
		return true
	}
	return false
}
