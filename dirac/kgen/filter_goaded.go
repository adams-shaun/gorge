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

// departedFilterRefusal refuses the filter words a battlefield-origin
// ChangesZone trigger (a dies / leaves trigger) cannot read the way gorge
// does: gorge matches the move on the last-known-information object.
//   - IsSuspected: the LKI record carries no arena designation bit.
//   - !blocking: isBlocking reads the live object's record and the attackers'
//     BlockedBy lists; the kernel's off-battlefield W7NotBlocking always holds.
//
// IsGoaded is NOT refused: gorge matches the move on the LKI object's goad
// records (trigmatch/zone.go's look-back pass reads the LKI's Goads and binds
// StaticallyGoadedWithLKI for the static route), and the kernel reads the goad
// the permanent still holds at its removal-point Dies firing (fireToGraveDies
// / fireLeave's Dies arm: the permanent is still on the battlefield with every
// characteristic the dies event reads). CR 603.10a's look-back, the same read
// on both sides.
func departedFilterRefusal(f *lFilter) error {
	switch {
	case filterReadsSuspected(f):
		return refuse("IsSuspected in a battlefield-origin ChangesZone ValidCard$")
	case filterReadsNotBlocking(f):
		return refuse("!blocking in a battlefield-origin ChangesZone ValidCard$")
	}
	return nil
}
