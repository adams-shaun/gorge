package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// commandCands is searchCands over the command zone (Command Beacon's
// "put your commander into your hand", gorge effHiddenPick with Origin$
// Command): the owner's commanders still in the command zone, in slot order
// (gorge's Game.Zone(ZCommand, owner) lists the partner pair in the order
// cmdGenesis seated them), through the pick's filter. The filter's YouOwn
// clause keeps the pool to the resolving controller's own commanders, the
// only ones that can match. A game with no commander has an empty pool.
func (s *State) commandCands(k *prep, in *kop.Instr, d *Decision) {
	if s.cfg&cfgCommander == 0 {
		return
	}
	p := s.g.pickP
	for j := 0; j < 2; j++ {
		o := s.cmdObj(p, j)
		if o == NoObj || s.objs[o].zone != kop.ZoneCommand || s.picksHas(o) {
			continue
		}
		if !s.pickOtherBarred(k, in.F, o) && s.matchCardGen(k, in.F, s.objs[o].card, p, s.res.e.ctrl, o) && s.pickFits(k, in.F, o, false) {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}
