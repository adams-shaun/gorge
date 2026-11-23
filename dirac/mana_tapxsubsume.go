package dirac

// The tap-another search's subsume run (gorge rules/potential_script.go
// potentialPrefixScript's third search, subsumeAnswers; the grant search's
// twin is mana_grantsubsume.go). Where the plain search ran out of its node
// budget and the play's base cost is coloured pips alone (scriptNeedCounts),
// gorge searches again with every colour ask narrowed to the colours the
// floating pool still lacks of that cost: a covered colour pays nothing more,
// so each line through it is dominated by the same line through a lacking
// one (CR 601.2g-h, 605.1a). General Tazri's {W}{U}{B}{R}{G} beside five
// Prism tokens under Diamond Kaleidoscope's "Sacrifice a Prism token: Add one
// mana of any color": the plain search spends its 160 nodes on the Prisms'
// repeated colours (five candidates by five colours per activation); the
// subsume run answers each ask with a colour still owed and finds the five
// activations in as many nodes.

// tapXSubsume runs the subsume search after a failed plain run of rs (rs holds
// that run's stall). It reports whether that run found a script, which rs.path
// then holds; false when gorge's conditions for the run do not hold (the plain
// run completed its space, or the play's cost is not coloured pips alone).
func (s *State) tapXSubsume(k *prep, a Action, rs *tapXSearch) bool {
	if !rs.limited {
		return false
	}
	cnt, ok := s.playCostCounts(k, a)
	if !ok {
		return false
	}
	rs.subsume, rs.costCol = true, cnt
	rs.nodes, rs.nseen, rs.n = 0, 0, 0
	t := s.fork()
	defer s.unfork()
	return rs.settle(k, t, false) && rs.n != 0
}

// subsumeKeeps is gorge's subsumeAnswers keep-rule for colour answer c of a
// multi-answer colour ask: in the subsume run only a colour the floating pool
// holds fewer of than the play's cost asks; every ask of the plain run keeps
// its answers whole.
func (rs *tapXSearch) subsumeKeeps(t *State, c uint8) bool {
	return !rs.subsume || rs.costCol[c] > t.pl[SeatIx(rs.p)].pool[c]
}
