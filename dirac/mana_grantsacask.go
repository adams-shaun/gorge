package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A granted sacrifice-only mana ability (Lotus Ring) is activated at priority,
// as a prefix step of a play (grantApply) or a step of the search on a fork
// (sacBranch). When two or more exile-instead effects compete for the
// sacrificed source's move (Rest in Peace twice, CR 616.1), gorge's script
// poses the replacement order ask inside the activation, before the mana
// colour, and its script answers the first option. The move cannot park for a
// play's prefix (nothing resumes after the ask), so the kernel takes the
// answer itself: the first option of exileOrderOptions, as the oracle's script
// does, and moves the source as that ask would have settled it.

// leaveAskedFirst moves sl to the graveyard, the order ask its move competes
// in answered with the first option.
func (s *State) leaveAskedFirst(k *prep, sl Slot) {
	o := s.perms[sl].obj
	if !s.park(k, o, parkLeave, 0) {
		s.leave(k, sl, kop.ZoneGraveyard, false) // no competition, or a queue that cannot hold it (dieZone stalls by name)
		return
	}
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkLeave && r.obj == o {
			r.kind = parkAnswered
			break
		}
	}
	mode := maxExMarks // the first source, after any marks
	for i := range s.cold.exMark {
		if m := &s.cold.exMark[i]; m.live && m.sl == sl {
			mode = i // a mark on the creature is listed before the sources
			break
		}
	}
	s.settleExileMarks(sl, mode)
	s.leave(k, sl, kop.ZoneGraveyard, false)
}
