package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpAtEOT's extended acts (kop.AtEOTDestroy and the others; C 0 and 1 are the
// sacrifice and the exile atEOTRider registers). Each is one gorge atEOTRiders
// row over the one battlefield object the op names:
//   - Destroy: {OpDestroy, next end step}, tracked (__kwAtEOTDestroy);
//   - Hand: {opDashReturn, next end step}, tracked (__kwDashReturn);
//   - SacCombat: {OpSacrifice, this turn's end of combat}, untracked
//     (__kwEncoreSacrifice, thisTurn);
//   - ExileCombat: {OpExile, this turn's end of combat}, tracked
//     (__kwWarpExile, thisTurn).
// The token riders of the same names are trig_tokenateot.go's.

// atEOTAct registers OpAtEOT's act c over object o.
//
//go:noinline
func (s *State) atEOTAct(o Obj, c int16) {
	if c <= 1 {
		s.atEOTRider(o, c == 1)
		return
	}
	d := delayed{ctrl: s.res.e.ctrl, obj: o, ver: objWord(s.objs[o].ver), creator: s.res.e.obj}
	switch c {
	case kop.AtEOTDestroy:
		d.act = uint8(kop.OpDestroy)
	case kop.AtEOTHand:
		d.act = opDashReturn
	case kop.AtEOTSacCombat:
		d.act, d.when = uint8(kop.OpSacrifice), dlyEndCombat
	case kop.AtEOTExileCombat:
		d.act, d.when = uint8(kop.OpExile), dlyEndCombat
	default:
		panic("kernel: OpAtEOT act not implemented")
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = d
	s.ndly++
}
