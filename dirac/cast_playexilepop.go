package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugPlayExilePopNoFilter makes an exile filter population (OpCastFree over
// every seat's exile, RefNone: dirac/kgen/lower_playexilepop.go) ignore its
// Valid$ filter (mutation).
var DebugPlayExilePopNoFilter bool

// castFreeFilterOn reports whether the OpCastFree instruction in tests its filter on a card
// of the population: always, but for the two mutations.
func castFreeFilterOn(in *kop.Instr, zone kop.Zone) bool {
	if in.F == 0 || DebugFreeFilterIgnored {
		return false
	}
	return !(DebugPlayExilePopNoFilter && zone == kop.ZoneExile && in.Obj == kop.RefNone)
}
