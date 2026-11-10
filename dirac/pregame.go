package dirac

// ChooseStarter makes p the starting player: the coin toss's winner chose
// to play or draw (the playdraw option). It reports whether it could, which is
// only before the mulligan round's first decision (Config.Mulligans > 0;
// without a mulligan round the first turn has already begun when Init
// returns). The opening hands already dealt stay; the mulligan round is
// posed starting player first, as Init poses it. It lives in this last
// file, out of line, so the hot files keep their layout.
func (s *State) ChooseStarter(p uint8) bool {
	if p >= s.nSeats() || s.mullMax == 0 || s.ctl != ctlMullDeclare || s.mullCur != 0 || s.Decisions != 0 {
		return false
	}
	s.startPl = p
	s.mullOrder = mullOrderFrom(p, s.nSeats())
	return true
}
