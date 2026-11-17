package effects

import "github.com/adams-shaun/gorge/engine/state"

// altCostKeywordPaid reports whether o was cast for the cost of its printed
// keyword head (Prowl, Freerunning): an alternative-cost row with no
// CastFlags bit of its own stamps state.FlagAltCostPaid (rules'
// altCastModes), and on a face carrying the keyword that bit names this
// keyword's cost, since CR 118.9a allows one alternative cost per cast and
// rules withholds the S:Mode$ AlternativeCost stamp from every face that does
// not read Count$AltCost. A nil object reads false.
func altCostKeywordPaid(o *state.Object, head string) bool {
	if o == nil || o.CastFlags&state.FlagAltCostPaid == 0 {
		return false
	}
	f := o.Face()
	if f == nil {
		return false
	}
	_, ok := f.KeywordParam(head)
	return ok
}
