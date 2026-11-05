package dirac

import "math/bits"

// The CR 616.1 order between an as-enters copy and another entry replacement
// for a clone that returns to the battlefield during a resolution (a mass
// return from exile under Deanna Troi's extra +1/+1 counter). gorge asks it
// as it does for a cast clone (repl_entryclone.go), with the card still in
// its zone, after the copy election (copy_cloneenter.go); the kernel keeps
// the parked return (raGenRetChoice) and poses the same KReplacement ask
// before it moves the card.
//
// Not modelled: a competition that gorge re-asks with the permanent already
// in (a remainder that does not commute) stalls (StallEntryOrder).

// cloneReturnOrderPark poses the order ask for the parked return of the clone
// s.g.pre.obj once the copy election is answered; false when no other source
// competes (the return goes on).
//
//go:noinline
func (s *State) cloneReturnOrderPark(k *prep) bool {
	p := &s.g.pre
	if k.gen == nil || p.ordSet || s.cloneOp(k, s.objs[p.obj].card) == nil {
		return false
	}
	c := s.cloneEntryComp(k, p.obj, s.res.askP)
	if c.over {
		s.stall(StallReplCands)
		return true
	}
	if c.n < 2 || c.n > 2 && !c.distinct() {
		return false
	}
	p.kind, p.state, p.ordSet, p.ordN = akEntryOrder, preReturn, true, 0
	return true
}

// answerReturnOrder takes an answer to the parked return's order ask: the
// named source applies next, then the return goes on with the picks.
//
//go:noinline
func (s *State) answerReturnOrder(k *prep, a Action) {
	p := &s.g.pre
	p.ordPick[p.ordN] = Obj(a.Source())
	p.ordN++
	c := s.cloneEntryComp(k, p.obj, s.res.askP)
	if rest := c.unpicked(p); bits.OnesCount8(rest) > 1 && !c.restCommutes(rest) {
		s.stall(StallEntryOrder)
		return
	}
	p.kind = akClone // the copy stays answered; ordPick names the order
	s.rerunReturn(k)
}
