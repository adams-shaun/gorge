package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// startingLife is each player's starting life total: 40 in a Commander game
// (CR 903.7), 20 otherwise (CR 119.1).
func (s *State) startingLife() int {
	if s.cfg&cfgCommander != 0 {
		return 40
	}
	return 20
}

// cmdGenesis puts seat p's commander, card idx of its deck (-1 none), into
// the command zone before the seat's shuffle and opening hand (CR 903.6), as
// gorge's genesis does: the commander leaves the ordered library, the rest
// keeping their relative order. An index outside the deck seats no commander
// (gorge skips it the same way).
func (s *State) cmdGenesis(p uint8, idx int16) {
	pl := &s.pl[p]
	if idx < 0 || int(idx) >= pl.libLen() {
		return
	}
	o := pl.lib[int(pl.libLo)+int(idx)]
	pl.libRemove(o)
	s.touch(o)
	s.objs[o].zone = kop.ZoneCommand
	pl.cmd = o
}
