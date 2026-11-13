package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// raTapOrUntapAll is OpExt ExtTapOrUntapAll's one election (res.askP the
// resolving controller).
const raTapOrUntapAll = raGenManifestDread + 83

// tapOrUntapAllStart is OpExt ExtTapOrUntapAll (gorge effTapOrUntapAll):
// the controller is always asked, before the sweep is read.
func (s *State) tapOrUntapAllStart() bool {
	s.res.ask, s.res.askP = raTapOrUntapAll, s.res.e.ctrl
	return false
}

// tapOrUntapAllOptions poses gorge's options in order: tap, untap; neither
// names an object.
func (s *State) tapOrUntapAllOptions(d *Decision) {
	d.Sub = ChGenTapOrUntapAll
	d.add(MakeAction(AkChoose, 0, 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// tapOrUntapAllAnswer runs the elected sweep over the instruction's filter:
// OpTapAll's walk (no player scope, the controller the tapper) or
// OpUntapAll's, then the program goes on.
func (s *State) tapOrUntapAllAnswer(k *prep, a Action) {
	s.res.ask = raNone
	in := s.prog(k)[s.res.pc]
	in.Obj, in.C = kop.RefNone, 0
	in.Op = kop.OpUntapAll
	if a.Mode() == 1 {
		in.Op = kop.OpTapAll
	}
	s.exec(k, &in)
	s.continueRes(k, 0)
}
