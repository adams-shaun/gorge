package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// FeatLandDropOpen reports whether p may still play a land this turn, the extra
// land drops of its permanents counted.
func (s *State) FeatLandDropOpen(p uint8) bool {
	return s.pl[p].landsPlayed < s.landLimit(s.k(), p)
}

// FeatCastDivideLeft is how many points of a cast's division are still to place.
func (s *State) FeatCastDivideLeft() int {
	if s.ctl != ctlBuild {
		return 0
	}
	return int(s.bld().divLeft)
}

// FeatTargetBounds is how many objects the target slot being asked takes in all, when
// the pending decision asks one that takes several.
func (s *State) FeatTargetBounds() (lo, hi int, ok bool) {
	if s.ctl != ctlBuild || s.bld().stage != bsTargets || !s.g.mt.active || s.g.mt.slot != s.bld().ti {
		return 0, 0, false
	}
	if t := tspecE(s.k(), &s.bld().e, int(s.bld().ti)); t == nil || !multiSlot(t) {
		return 0, 0, false
	}
	return int(s.g.mt.min), int(s.g.mt.max), true
}

// FeatEnterSource is the permanent an as-enters choice or an entering replacement's
// order is being asked for.
func (s *State) FeatEnterSource() Obj {
	resolving := s.ctl == ctlResolve
	switch {
	case s.g.pre.state == 1 || resolving && (s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice):
		return s.g.pre.obj
	case resolving && s.res.ask == raGenEntryOrder || s.ctl == ctlGenAsk && s.entryAsked(s.k()):
		return s.g.ro.entObj()
	case s.g.nask == 0 || s.ctl != ctlGenAsk && !(resolving && s.res.ask == raGenEnterAsk):
		return NoObj
	}
	if a := &s.g.ask[0]; a.sl != NoSlot {
		return s.perms[a.sl].obj
	}
	return Obj(s.g.ask[0].ab) // an Aura coming back from exile
}

// FeatHandCost is what p pays to cast hand card o as printed, its cost statics applied
// but none that depends on what the spell targets, and whether they changed it.
func (s *State) FeatHandCost(p uint8, o Obj) (kop.ManaCost, bool) {
	k := s.k()
	c := s.objs[o].card
	cost := k.cards[c].cost
	if !s.statM.any() && k.gen == nil {
		return cost, false
	}
	was := s.g.noTgtRed
	s.g.noTgtRed = true
	got := s.featCost(k, p&1, c, o, cost)
	s.g.noTgtRed = was
	return got, got != cost
}

// DebugStall stops the game at a declared limit, the way a full pool does.
func (s *State) DebugStall(why uint8) { s.capFull(why) }

// FeatResolving reports whether the pending decision is asked inside a resolution.
func (s *State) FeatResolving() bool {
	return s.ctl == ctlResolve || s.g.pre.state == 1 || s.ctl == ctlGenAsk && s.g.ctlRet == ctlResolve
}

// FeatLifeThisTurn is what p gained and lost this turn.
func (s *State) FeatLifeThisTurn(p uint8) (gained, lost int32) {
	return int32(s.pl[p&1].lifeGained), int32(s.pl[p&1].lifeLost)
}
