package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwEnteredFrom: FwEnteredFrom, the object was put into the zone it is in now
// from zone Arg this turn (gorge engine/effects/filter_word.go
// wordThisTurnEnteredFrom: EnteredThisTurn && EnteredFrom == the zone). The
// answer is the entered ledger's latest entry naming the object
// (enteredFrom, filter_enteredfrom.go); a candidate with no object reads
// false, and a table without the ledger keeps none (the table reads the word,
// so tableHasEnteredFrom turns it on).
//
//kmanifest:fword FwEnteredFrom
func (s *State) fwEnteredFrom(c *fctx, l *kop.FLit) bool {
	return c.o != NoObj && s.enteredFrom(c.o, kop.Zone(l.Arg))
}

// progHasEnteredFrom reports whether a filter program of t reads FwEnteredFrom.
func progHasEnteredFrom(t *kop.Table) bool {
	for i := range t.FilterProgs {
		for _, l := range t.FilterProgs[i].Lits {
			if l.W == kop.FwEnteredFrom {
				return true
			}
		}
	}
	return false
}
