package dirac

// sharesCrCmdr is kop.PredSharesCrCmdr for object o: o shares a creature type
// with a commander of seat ctrl, the evaluating controller (gorge
// sharesCreatureTypeWith Commander, effects sharesTypeReferentsCommander: the
// resolving source's controller's commanders, both partner slots). Each side
// reads its creature types where it now is (crTypesOf: a commander in the
// command zone its printed ones, on the battlefield its current ones), and one
// with every creature type shares with any object that has a creature type
// (CR 702.73a). A seat with no commander matches nothing.
func (s *State) sharesCrCmdr(k *prep, o Obj, ctrl uint8) bool {
	return s.sharesCrCmdrRead(k, o, ctrl) != DebugSharesCrCmdrFlip
}

func (s *State) sharesCrCmdrRead(k *prep, o Obj, ctrl uint8) bool {
	if o == NoObj || s.cfg&cfgCommander == 0 {
		return false
	}
	om, oa := s.crTypesOf(k, o)
	for j := 0; j < 2; j++ {
		c := s.cmdObj(ctrl, j)
		if c == NoObj {
			continue
		}
		cm, ca := s.crTypesOf(k, c)
		if s.sharesCr(om, oa, cm, ca) {
			return true
		}
	}
	return false
}

// DebugSharesCrCmdrFlip is the mutation switch: every kop.PredSharesCrCmdr
// answer flips. Test-only and cold.
var DebugSharesCrCmdrFlip bool
