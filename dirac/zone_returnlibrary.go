package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// returnSkipsLibrary reports whether a return of an object in zone z must
// not move it: Origin$ Library gates the move to a library card, and every
// other return skips one (a card shuffled away is a new object, CR 400.7).
// The exception is an OpReturnEx for Origin$ All on a remembered card
// (kop.MvEntryRiders' bit, the flag's second reading), which takes a library
// card too (Turntimber Symbiosis's dig pick).
func returnSkipsLibrary(in *kop.Instr, z kop.Zone) bool {
	if in.B.N&kop.RetFromLibrary != 0 {
		return z != kop.ZoneLibrary
	}
	fromAny := in.Op == kop.OpReturnEx && in.C>>3&kop.MvEntryRiders != 0
	return z == kop.ZoneLibrary && !fromAny
}
