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

// answerOrderAsksFirst answers the order asks the state-based check after the
// sacrifice parked (a second creature of the board dying to the lost bonus,
// CR 704.3) with the first option each, as the script does, and runs the check
// again after each answer as the priority step would. The answers are no
// decisions of the play.
func (s *State) answerOrderAsksFirst(k *prep) {
	n := s.Decisions
	for guard := 0; guard < 8 && s.ctl == ctlExileOrder && !s.over; guard++ {
		var d Decision
		s.exileOrderOptions(k, &d)
		if d.N == 0 {
			break
		}
		s.Apply(d.Options[0])
		if s.ctl == ctlPriority {
			s.sba(k)
		}
	}
	s.Decisions = n
}
