package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// endTurnExile is end-the-turn's CR 724.1a-b: the triggered abilities
// waiting to be put on the stack cease to exist, THEN every other spell and
// ability on the stack is exiled (the resolving entry is the top; that card
// is exiled as it finishes). The pending triggers are dropped first, as
// gorge does (emit.go's EndTurn arm), so an ability that triggers
// during the process (the exile) survives into the cleanup step (CR
// 724.1f).
func (s *State) endTurnExile() {
	s.npend = 0
	for s.sp > 1 {
		x := s.stack[0]
		copy(s.stack[:s.sp-1], s.stack[1:s.sp])
		s.sp--
		if x.kind == seSpell {
			s.touch(x.obj)
			ob := &s.objs[x.obj]
			ob.zone = kop.ZoneNone
			s.placeFrom(x.obj, kop.ZoneExile, false, kop.ZoneStack, false)
		}
	}
}
