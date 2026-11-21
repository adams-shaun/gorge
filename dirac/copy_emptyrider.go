package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// delayEmptyRider is execCopyToken's arm for a copy of nothing (Mordor on the
// March's "Exile a creature card from your graveyard. Create a token that's a
// copy of it ... Exile it at the beginning of the next end step" with no
// creature card to exile). No token is made, but the AtEOT$ rider's delayed
// triggered ability is created all the same (CR 603.7): it fires at the end
// step (SacrificeUpkeep: the upkeep) and does nothing, as gorge's
// copypermanent.go Remembered arm registers it through scheduleAtEOTNone.
//
//go:noinline
func (s *State) delayEmptyRider(in *kop.Instr) {
	if in.B.N&(kop.CpSacAtEndDly|kop.CpExileAtEnd) == 0 {
		return
	}
	if sacAtUpkeepOf(in.B.N) {
		s.delayEmptyUpkeep(&s.res.e)
	} else {
		s.delayEmpty(&s.res.e)
	}
}
