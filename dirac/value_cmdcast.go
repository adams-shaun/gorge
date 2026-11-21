package dirac

// cmdCastTotal is kop.ExTotalCommanderCastFromCommandZone: seat p's casts of
// its own commanders from the command zone, both commanders of a partner
// pair together (CR 903.8), saturating as the per-commander tax counter does.
// gorge's Host.CommanderCastsFromCommandZone counts the same PutOnStack
// events the tax reads; a seat with no commander slots (a non-Commander game,
// an empty Commanders list) reads 0.
func (s *State) cmdCastTotal(p uint8) int {
	if s.cfg&cfgCommander == 0 {
		return 0
	}
	p = SeatIx(p)
	n := 0
	for j := 0; j < 2; j++ {
		if DebugCmdCastCountFirstOnly && j > 0 {
			break // mutation: a partner's casts are not read
		}
		if s.cmdObj(p, j) != NoObj {
			n += int(*s.cmdCastAt(p, j))
		}
	}
	n = min(n, 255)
	if st := DebugCmdCastStats; st != nil {
		st.Reads++
		st.Max = max(st.Max, n)
		if n >= 2 {
			st.AtLeast2++
		}
	}
	return n
}

// CmdCastStats counts the reads of cmdCastTotal (test hook).
type CmdCastStats struct {
	Reads    int // expression evaluations
	AtLeast2 int // evaluations that read two or more casts
	Max      int // the largest value read
}

// DebugCmdCastStats is the test hook; nil in normal play.
var DebugCmdCastStats *CmdCastStats

// DebugCmdCastCountFirstOnly (TestCommanderCastCountMutation): the total
// reads only slot 0, so a partner's casts go uncounted.
var DebugCmdCastCountFirstOnly bool
