package kgen

// goadedWord lowers IsGoaded / !IsGoaded (CR 701.15b; gorge
// engine/effects/filter.go matchPositive's IsGoaded): the candidate is goaded
// by ANY player, the event-backed goad relationships and a live Goad$ True
// static alike, read by the kernel's goadedAt (dirac/act_goad.go). n is 1 for
// the positive word and 0 for the negation. The predicate is battlefield-only:
// off the battlefield nothing is goaded, so the negated word is true there
// (PredCardCold).
func (c *lClause) goadedWord(n int) error {
	return c.setCmp(cmpGoaded, 0, n, "", "")
}

// filterReadsGoaded reports whether any clause of f is the IsGoaded word
// (either sign).
func filterReadsGoaded(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.CmpStat == cmpGoaded {
			return true
		}
	}
	return false
}

// departedFilterRefusal refuses the filter words a battlefield-origin
// ChangesZone trigger (a dies / leaves trigger) cannot read the way gorge
// does: gorge matches the move on the last-known-information object.
//   - IsSuspected: the LKI record carries no arena designation bit.
//   - IsGoaded: gorge clears Object.Goads on the zone move, so its match reads
//     the departed creature as not goaded while the kernel reads the goad it
//     still held (CR 603.10a would fire the trigger; gorge first).
//   - !blocking: isBlocking reads the live object's record and the attackers'
//     BlockedBy lists; the kernel's off-battlefield W7NotBlocking always holds.
func departedFilterRefusal(f *lFilter) error {
	switch {
	case filterReadsSuspected(f):
		return refuse("IsSuspected in a battlefield-origin ChangesZone ValidCard$")
	case filterReadsGoaded(f):
		return refuse("IsGoaded on a departed object: gorge clears Goads on the zone move")
	case filterReadsNotBlocking(f):
		return refuse("!blocking in a battlefield-origin ChangesZone ValidCard$")
	}
	return nil
}
