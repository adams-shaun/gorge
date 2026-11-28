package dirac

// The plain RepeatEach loop over the chosen hand cards (RepeatCards$
// Card.ChosenCard | Zone$ Hand, header Obj kop.RefChosen: Sylvan Library).
// Gorge's repeatedCards walks every hand from the resolving controller's for
// the cards the resolution chose (c.Chosen), so the subjects are the chosen
// cards still in a hand, in hand order. With ChooseOrder$ True
// (kop.RepeatEachOrder beside kop.RepeatPlain) the controller orders them
// before the first pass (act_eachorder.go), then the resumable frame
// (repeatEachCardsFrame) runs the passes in that order: the frame holds the
// subjects after the first as g.chosen indices, so eachChosenFront first
// moves the subjects to the front of g.chosen in loop order. g.chosen is a
// set to every other reader (F2ChosenCard, the body's hand pick), so the
// reorder changes nothing else. A subject that has left the hand by its turn
// still runs its pass, as gorge's fixed subject list does.

// eachChosenList lists the chosen cards in a hand, each seat's hand from the
// resolving controller's, in hand order.
func (s *State) eachChosenList(list *[MaxPerm]Obj) int {
	n := 0
	q := s.aliveFrom(s.res.e.ctrl)
	for i := uint8(0); i < q.n; i++ {
		pl := &s.pl[q.s[i]]
		for _, o := range pl.hand[:pl.handN] {
			if n < len(list) && s.inSet(&s.g.chosen, s.g.nchosen, o) {
				list[n] = o
				n++
			}
		}
	}
	return n
}

// eachChosenFront puts the loop's subjects (list, in pass order) first in
// g.chosen and sets rem to the indices of every subject after the first.
func (s *State) eachChosenFront(list []Obj, rem *slotMask) {
	var out [maxRem]Obj
	m := 0
	for _, o := range list {
		if m < len(out) {
			out[m] = o
			m++
		}
	}
	for _, o := range s.g.chosen[:s.g.nchosen] {
		in := false
		for _, x := range list {
			in = in || x == o
		}
		if !in && m < len(out) {
			out[m] = o
			m++
		}
	}
	s.g.chosen, s.g.nchosen = out, uint8(m)
	for i := 1; i < len(list) && i < m; i++ {
		*rem = rem.with(Slot(i))
	}
}
