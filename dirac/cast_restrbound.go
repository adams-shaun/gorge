package dirac

// restrBoundDrops: on the any-mana walk (an unrelaxable census: every play
// gorge's potential walk lists is searched), play a is not one the walk lists.
// gorge prices each play against the potential bound built for that payment
// (potentialPoolFor, potentialRestrictionAdmits, CR 106.6): a restricted
// source whose RestrictValid$ refuses the spell adds nothing to it. The open
// walk counts every source's mana for every play, so a Jeweled Lotus (three
// mana of one colour, only to cast its owner's commander) opened the bound for
// an Ordeal of Thassa that Urza's once-counted tap-an-artifact mana could not
// reach. A bound that is closed once the payment's restrictions apply, and
// does not afford the cast, lists no such play.
func (s *State) restrBoundDrops(k *prep, p uint8, a Action) bool {
	if !isCastKind(a.Kind()) || a.Mode() != 0 {
		return false
	}
	var open avail
	if !s.availPotentialFor(k, p, &open, nil, false) {
		return false // the bound is closed for every payment: the gaps walk's plays
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	d := s.payDescNow(o)
	var av avail
	if s.availPotentialFor(k, p, &av, &d, false) {
		return false // open for this payment too: an unrestricted open source
	}
	cost := s.adjustFit(k, &av, p, c, o, k.cards[c].cost, true)
	return !av.affords(&cost, pipMaskOf(&cost), 0)
}
