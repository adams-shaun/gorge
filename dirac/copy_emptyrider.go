package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// delayEmptyRider is the copy ops' arm for a copy of nothing -- Mordor on the
// March's empty remembered set ("Exile a creature card from your graveyard.
// Create a token that's a copy of it ... Exile it at the beginning of the next
// end step"), Dominion Infiltration's X = 0 or a target that left before
// resolution, a CreateToken rewriter, or an Aura copy with no legal host. No
// token is made, but the AtEOT$ rider's delayed triggered ability is created
// all the same (CR 603.7a): it fires at the end step (SacrificeUpkeep: the
// upkeep) and does nothing, as gorge's copypermanent.go tail registers it
// through scheduleAtEOTNone for every grouped rider.
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
