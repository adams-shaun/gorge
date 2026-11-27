package dirac

// imprintPlayedCommit is ImprintPlayed$ True (gorge rules/play_tape.go's
// imprintPlayed: every card the Play actually BEGINS to play joins the
// resolution source's Imprinted list). It runs where ForgetPlayed$ and
// RememberPlayed$ do, as the cast being built commits on the stack (CR
// 601.2a): a decline, a refused cast or a cast reversed for an unpayable cost
// (CR 733.1-2) never reaches it and imprints nothing. o is the entry's
// object, the minted copy for a CopyCard$ play. Unlike RememberPlayed$ (which
// keeps gorge's first-card-only ctx.Play record), every begun card is
// imprinted, so the freeP bit is not spent here and an Amount$ Play of
// several cards imprints each.
func (s *State) imprintPlayedCommit(o Obj) {
	if s.g.freeP&freeImprint == 0 || s.bld().bfl&bflPushed == 0 {
		return
	}
	s.imprint(o)
}
