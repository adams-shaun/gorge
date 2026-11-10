package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// FeatLandLimit is how many lands p may play in a turn, the extra land drops
// included.
func (s *State) FeatLandLimit(p uint8) uint8 {
	return s.landLimit(s.k(), p)
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

// FeatEnterSource is the object an ask is about, when it is not the resolving one: a
// permanent's as-enters choice or entering replacements' order, what an entering Aura
// will enchant, or the replacements' order for a move.
func (s *State) FeatEnterSource() Obj {
	resolving := s.ctl == ctlResolve
	switch {
	case s.ctl == ctlExileOrder:
		return s.cold.parked[0].obj
	case resolving && s.res.ask == raGenEnchant:
		return s.enchantingObj(s.k())
	case s.preAskPosed() || resolving && (s.res.ask == raGenTokChoice || s.res.ask == raGenRetChoice):
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

// FeatResolving reports whether the pending decision is asked inside a resolution.
// A held free cast and a queued as-enters choice count: gorge makes them there, where
// the kernel makes them just after (act_discoverfreecast.go, turn_gen.go).
func (s *State) FeatResolving() bool {
	return s.ctl == ctlResolve || s.g.pre.state == 1 || s.ctl == ctlGenAsk && (s.g.ctlRet == ctlResolve || s.g.priSave != 0) || s.g.hold
}

// FeatLifeThisTurn is what p gained and lost this turn.
func (s *State) FeatLifeThisTurn(p uint8) (gained, lost int32) {
	return int32(s.pl[SeatIx(p)].lifeGained), int32(s.pl[SeatIx(p)].lifeLost)
}

// ObjVer counts object o's zone changes (CR 400.7). It wraps at 256.
func (s *State) ObjVer(o Obj) uint8 { return s.objs[o].ver }

// FeatDivision is the pending damage division: the blockers in block order, and how
// much the attacker divides among them. SplitAt reads an option's split from these.
func (s *State) FeatDivision(out *[MaxPerm]Obj) (n, damage int) {
	var bl [MaxPerm]Slot
	n = s.liveBlockers(s.divA, &bl)
	for i := range n {
		out[i] = s.perms[bl[i]].obj
	}
	return n, int(s.divRem)
}

// FeatBlockOrder is where blocking permanent o was declared among its attacker's
// blockers.
func (s *State) FeatBlockOrder(o Obj) uint8 { return s.perms[s.objs[o].slot].bord }

// FeatDamageOp is what the n-th damage instruction of the ability being built or
// resolved deals now, read for a target ask: X and kicks are known by then (CR 601.2b).
func (s *State) FeatDamageOp(n int) (int, bool) {
	e := s.curEntry()
	if e == nil {
		return 0, false
	}
	k := s.k()
	if int(e.card) >= len(k.t.Cards) || int(e.ab) >= len(k.t.Cards[e.card].Abilities) {
		return 0, false
	}
	prog := k.t.Cards[e.card].Abilities[e.ab].Prog
	for i := range prog {
		switch prog[i].Op {
		case kop.OpDealDamage, kop.OpDealDamageEx, kop.OpDealDamageDivided, kop.OpDealDamageFrom, kop.OpDamageAll:
			if n == 0 {
				return s.evalV(k, prog[i].A, s.ctxCur()), true
			}
			n--
		}
	}
	return 0, false
}
