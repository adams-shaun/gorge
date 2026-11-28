package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An unless or ward cost's offer gate beside a once-each-turn Combo source
// (Vivi Ornitier's {0}). gorge offers "pay" only when the sources its unless
// census prices reach the cost (rules paid_mana_census.go unlessWindowUnits,
// read by pay.UnlessCostValuePayable), and that census lists no unit for an
// ability with an activation limit whose cost is no bare {T}. So the source
// funds no offer, alone or beside other sources. CR 605.3a would allow the
// activation: the omission is gorge's, mirrored here until its census prices
// the shape. The window itself, once other sources open it, lists every
// source with a usable ability (rules ward.go wardManaDecision), this one
// included, so only the gates read the mark.

// enterUnlessCensus marks the walk as pricing an unless / ward offer
// (ForkPool.unlessGate) and returns the mark to hand back to
// leaveUnlessCensus.
func (s *State) enterUnlessCensus() bool {
	if s.pool == nil {
		s.pool = new(ForkPool) // (fork makes it the same way)
	}
	was := s.pool.unlessGate
	s.pool.unlessGate = true
	return was
}

// leaveUnlessCensus restores the mark enterUnlessCensus returned.
func (s *State) leaveUnlessCensus(was bool) { s.pool.unlessGate = was }

// unlessCensusSkips reports whether Combo ability a is no unit of the unless
// census being priced.
func (s *State) unlessCensusSkips(a *kop.Ability) bool {
	return s.pool != nil && s.pool.unlessGate && a.Cost.Flags&kop.CostOncePerTurn != 0
}

// canPayUnlessGen is canPayGen as an unless cost's offer gate reads it.
func (s *State) canPayUnlessGen(k *prep, p uint8, kind int, amt int, pips int16, f kop.FilterID) bool {
	was := s.enterUnlessCensus()
	ok := s.canPayGen(k, p, kind, amt, pips, f)
	s.leaveUnlessCensus(was)
	return ok
}
