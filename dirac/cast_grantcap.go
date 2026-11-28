package dirac

// cappedScriptWalk is priorityOptions' script walk behind grantCap's cap: the
// plays the grant search refused within gorge's budget, which scriptOffers
// must not price again. Only a table with a grant search, on a walk with no
// availX mode, builds the cap; every other priority walks with a nil cap (an
// empty one: scriptOfferDrops), so it does not clear a Decision it never
// fills. The cap lives on this frame: prep is the table's shared, read-only
// value, so it holds no per-call scratch.
func (s *State) cappedScriptWalk(k *prep, d *Decision, p uint8, sorc bool) {
	if len(k.gen.grantPC) == 0 || s.g.availX != 0 {
		s.scriptWalk(k, d, nil, p, sorc)
		return
	}
	var capSpent Decision
	s.grantCap(k, d, p, &capSpent)
	s.scriptWalk(k, d, &capSpent, p, sorc)
}
