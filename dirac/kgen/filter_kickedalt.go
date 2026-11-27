package kgen

// The kicker-alternative words of a two-cost Kicker card (CR 702.33b; gorge
// engine/effects/filter_word.go wordKickedIndex over Object.CastFlags
// FlagKicked1 / FlagKicked2): "kicked 1" and "kicked 2" hold when that
// alternative was paid. Both lower to kop.PredKickedAlt, PredN the index,
// read on the battlefield, or on the spell itself as a SpellCast self
// trigger's filter (dirac/trig_selfcast.go selfCastFilterOK; selfCastClause);
// build.go refuses the word elsewhere. The bare "kicked" word is a different clause flag
// (fKicked) and never reaches here.
const (
	kickedAltFirst  = "kicked 1"
	kickedAltSecond = "kicked 2"
)

// kickedAltProp reads p as one of the two words; ok is false for any other.
func (c *lClause) kickedAltProp(p string) (ok bool, err error) {
	switch p {
	case kickedAltFirst:
		return true, c.setCmp(cmpKickedAlt, 0, 1, "", "")
	case kickedAltSecond:
		return true, c.setCmp(cmpKickedAlt, 0, 2, "", "")
	}
	return false, nil
}
