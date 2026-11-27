package policynet

import (
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// SplitOmniscientRecord is SplitOmniscientView over a view.OmniscientRecord:
// the seat's view with the other hands removed, plus a Diag that also carries
// library order -- up to depth card names off the top of the seat's own
// library, and of every other seat's library in player order (each top
// first). depth <= 0 records no library tops, which is exactly
// SplitOmniscientView's Diag. A record only exists for a non-seat caller
// (view.ProduceOmniscientRecord), so this never feeds a playing seat.
func SplitOmniscientRecord(rec view.OmniscientRecord, seat state.PlayerID, depth int) (view.View, *Diag) {
	v, diag := SplitOmniscientView(rec.View, seat)
	if depth <= 0 {
		return v, diag
	}
	diag.OwnLibraryTop = []string{}
	diag.OppLibraryTop = []string{}
	for i, lib := range rec.Libraries {
		dst := &diag.OppLibraryTop
		if i < len(rec.View.Players) && rec.View.Players[i].ID == seat {
			dst = &diag.OwnLibraryTop
		}
		for j := 0; j < depth && j < len(lib); j++ {
			*dst = append(*dst, lib[j].Name)
		}
	}
	return v, diag
}
