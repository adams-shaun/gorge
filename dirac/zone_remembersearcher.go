package dirac

// rememberSearcher is kop.ExtRememberSearcher (gorge effSearchLibrary:
// RememberSearcher$ remembers the player who searches, found card or not). The
// kernel cannot hold a player in the remembered set, so the searcher joins the
// remembered count instead; kgen lowers the flag only for a single search by
// the controller whose remembered set is read through its size.
func (s *State) rememberSearcher() {
	s.g.remCnt++
}
