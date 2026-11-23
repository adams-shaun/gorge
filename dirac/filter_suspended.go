package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwSuspended: FwSuspended, gorge's `suspended` read: the compiled matcher
// consults predicates["suspended"] (engine/effects/filter.go:637) before the
// word table, so wordSuspended (filter_word.go:919, the CastFlags&FlagSuspend
// read) never decides: the object is in exile with a positive TIME counter
// and is a Suspend card (a suspend-action flag, an event-backed grant, or the
// printed keyword). The kernel's only time counters on an exiled card are its
// DurSuspend record's (kw_suspend.go suspendTime), kept for a card suspended
// by the special action and for one a spell's own exile suspended (CR
// 702.62b); the card is a Suspend card either way. A queued or declined card
// has no counter left and reads false, as in gorge.
//
//kmanifest:fword FwSuspended
func (s *State) fwSuspended(c *fctx, l *kop.FLit) bool {
	if !c.card() || c.o == NoObj || c.zone != kop.ZoneExile {
		return false
	}
	return s.suspendTime(c.o) > 0
}
