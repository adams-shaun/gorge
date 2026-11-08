package dirac

// addCostTargetAbort: a spell whose additional pick cost (a discard, a sacrifice, a
// tap, life) can no longer be paid has no target on gorge's menu, since
// affordableTargetCandidates keeps a candidate only when nonManaCastable passes, so a
// required target reverses the cast there (CR 601.2c, 733.1). A Play's free cast is
// begun without the offer's cost census, so this is where gorge first meets it:
// Wishing Well casting Sazacap's Brew from a graveyard with the hand empty.
func (s *State) addCostTargetAbort(k *prep) bool {
	b := s.bld()
	return b.e.kind == seSpell && !k.t.Cards[b.e.card].HasAltAdd && !s.spellCostsFeasible(k, b.e.card, b.e.ctrl, b.e.obj)
}
