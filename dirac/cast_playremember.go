package dirac

// DebugNoRememberPlayed skips RememberPlayed$'s write (mutation: a Remembered
// reader such as Chandra's "if you don't" damage then sees an empty set).
var DebugNoRememberPlayed bool

// rememberPlayedCommit is RememberPlayed$ True (gorge effects/play.go's
// PlayDone pass: rememberTarget on the object whose play began). It runs where
// ForgetPlayed$ does, as the cast being built commits on the stack (CR 601.2a):
// a decline, a refused cast or a cast reversed for an unpayable cost (CR
// 733.1-2) never reaches it and remembers nothing. o is the entry's object, the
// minted copy for a CopyCard$ play. An Amount$ Play that begins several cards
// remembers the first only (gorge's ctx.Play holds one card): the freeP bit is
// spent by the first write. remember also marks the resolving source's
// persistent list (mechRem) and its SrcMem copy.
func (s *State) rememberPlayedCommit(o Obj) {
	if !s.freePlayCast() || s.g.freeP&freeRemember == 0 || s.bld().bfl&bflPushed == 0 {
		return
	}
	s.g.freeP &^= freeRemember
	if DebugNoRememberPlayed {
		return
	}
	s.remember(o)
}
