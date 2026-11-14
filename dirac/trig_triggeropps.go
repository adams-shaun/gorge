package dirac

// triggerOpps is RefTriggerOpponents: gorge's Defined$ TriggeredCardOpponent,
// AliveFrom(controller) without the trigger's player (engine/effects
// definedSpecTriggeredCardOpponent). A trigger that names no player (255)
// names nobody, never the source's controller.
func (s *State) triggerOpps() seatSeq {
	e := &s.res.e
	if e.trigPl == 255 {
		return seatSeq{}
	}
	q := s.aliveFrom(e.ctrl)
	out := seatSeq{}
	for i := uint8(0); i < q.n; i++ {
		if q.s[i] != e.trigPl {
			out.add(q.s[i])
		}
	}
	return out
}
