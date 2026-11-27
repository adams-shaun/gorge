package dirac

// A batched RepeatEach over cards with ChooseOrder$ True (Ezuri's Predation):
// the controller orders the subjects before the loop runs. Gorge's
// repeatEachChooseOrder asks one KChoose of "order" options over the whole
// subject list (Min=Max=the count); the kernel asks one pick per decision and
// keeps the chosen permutation in res.arr, so the loop's one zone batch opens
// only after the last pick (kop.RepeatEachOrder; act_repeatcards.go).

// eachOrderMarker is res.chosen's sentinel while an order pick is in progress
// (no object uses it; RefChosen reads none because no body runs until the
// permutation is complete).
const eachOrderMarker objWord = 0xffff

// eachOrderPicked reports whether o is already in the chosen permutation.
func (s *State) eachOrderPicked(o Obj) bool {
	r := &s.res
	for i := uint8(0); i < r.topN; i++ {
		if r.arr[i] == o {
			return true
		}
	}
	return false
}

// eachOrderStart begins or continues the ordering of an OpRepeatEach's
// subjects. It reports true when the resolution must park on the next order
// pick; false when the permutation is complete and list has been reordered.
func (s *State) eachOrderStart(list []Obj, n int) bool {
	r := &s.res
	if r.chosen != eachOrderMarker {
		if n > len(r.arr) {
			s.capFull(StallRemember) // more subjects than the permutation carrier holds
			return true
		}
		r.chosen = eachOrderMarker
		r.left, r.topN = uint8(n), 0
		r.askP = r.e.ctrl
	}
	if r.left == 0 {
		s.eachOrderApply(list, n)
		return false
	}
	r.ask = raGenChooseCard
	return true
}

// eachOrderApply reorders the subject list by the chosen permutation and ends
// the order state. A subject that is no longer in the list would mean a zone
// move during the pick, which cannot happen; it stalls loudly.
func (s *State) eachOrderApply(list []Obj, n int) {
	r := &s.res
	var out [MaxPerm]Obj
	cnt := 0
	for i := uint8(0); i < r.topN && int(i) < len(r.arr); i++ {
		o := r.arr[i]
		for j := 0; j < n; j++ {
			if list[j] == o {
				out[cnt] = o
				cnt++
				break
			}
		}
	}
	r.chosen = 0
	r.left, r.topN = 0, 0
	if cnt != n {
		s.capFull(StallRemember)
		return
	}
	copy(list, out[:n])
}

// eachOrderOptions poses the next order pick: every subject not yet chosen,
// as gorge's "order" options. It is the raGenChooseCard ask's RepeatEach
// arm (chooseCardOptions).
func (s *State) eachOrderOptions(k *prep, d *Decision) {
	d.Sub = ChGenEachOrder
	in := &s.prog(k)[s.res.pc]
	var list [MaxPerm]Obj
	n := s.eachCardList(k, in, &list)
	for i := 0; i < n; i++ {
		o := list[i]
		if s.eachOrderPicked(o) {
			continue
		}
		d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	}
	if d.N == 0 {
		s.capFull(StallRemember)
	}
}

// eachOrderAnswer records one pick. With picks left the next decision is
// posed by the engine; the last pick runs the loop and continues the program.
// It is the raGenChooseCard ask's RepeatEach arm (applyChooseCard).
func (s *State) eachOrderAnswer(k *prep, a Action) {
	r := &s.res
	if r.chosen != eachOrderMarker || r.left == 0 || int(r.topN) >= len(r.arr) || s.eachOrderPicked(Obj(a.Source())) {
		s.capFull(StallRemember)
		return
	}
	r.arr[r.topN] = Obj(a.Source())
	r.topN++
	r.left--
	if r.left > 0 {
		return // leave r.ask set: the next pick is posed
	}
	r.ask = raNone
	in := &s.prog(k)[r.pc]
	s.repeatEachCards(k, in)
	if s.g.stalled != 0 {
		return
	}
	r.pc++ // the op set pc to the last body instruction
	s.runProg(k)
}
