package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The filter words ThisTurnEnteredFrom_Battlefield (kop.W7EnteredFromBF) and
// ThisTurnEnteredFrom_Library (kop.W7EnteredFromLib): gorge reads the
// object's latest arrival into its current zone, so the answer is the last
// entered-ledger entry naming the object.

// tableHasEnteredFrom reports whether any filter of t carries the word, which
// makes the table record the entered ledger even without an Expr reader.
func tableHasEnteredFrom(t *kop.Table) bool {
	for i := range t.Filters {
		if f := &t.Filters[i]; f.Pred == kop.PredWord7 && (f.PredN&0xff == kop.W7EnteredFromBF || f.PredN&0xff == kop.W7EnteredFromLib) {
			return true
		}
	}
	return false
}

// enteredFrom reports whether o was put into the zone it is in now from zone
// from this turn. A full ledger already stalled, so the list is never short
// here.
//
//go:noinline
func (s *State) enteredFrom(o Obj, from kop.Zone) bool {
	if !s.g.enteredOn || !s.g.zoneTurn.has(o) {
		return false
	}
	for i := int(s.g.enteredN) - 1; i >= 0; i-- {
		if e := &s.cold.entered[i]; e.o == o {
			return e.to() == uint8(s.objs[o].zone) && e.from() == uint8(from)
		}
	}
	return false
}
