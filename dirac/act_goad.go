package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// goad (CR 701.15; gorge effects/misc.go effGoad, events
// foldGoad / expireTurnGoads, rules/combat act_goad.go and attack.go's
// RequirementSet.Goad). The default duration only: a permanent goaded by
// seat p stays goaded until p's next turn begins (gorge's TurnChange fold;
// a departed goader's goad stays: expireTurnGoads only fires on its own
// turn and pruneGoads ignores the loss) or it leaves the battlefield (a new
// object). A goaded creature attacks each combat if able (CR 701.15b; gorge
// marks the attack option Required) and attacks a player other than its
// goaders if able: attackOffersN offers a goader as defender only when every
// other live player goaded it too (gorge GoadMayAttack), and gorge's CR
// 508.1d filter (attackOffers: a pair satisfying fewer player-attack duties
// than the creature's best) drops its planeswalker and battle offers. Cold
// and out of line: a table with no OpGoad never takes these paths
// (genPrep.anyGoad).

// DebugIgnoreGoad makes the kernel read every permanent as not goaded. The
// lockstep mutation test flips it and requires a divergence.
var DebugIgnoreGoad bool

// prepGoad sets anyGoad: some program goads, or a static goads
// (kop.StatGoad, anyStatGoad).
func prepGoad(t *kop.Table, g *genPrep) {
	g.anyStatGoad = tableHasStatic(t, kop.StatGoad)
	g.anyGoad = g.anyStatGoad
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				g.anyGoad = g.anyGoad || in.Op == kop.OpGoad
			}
		}
	}
	if MaxSeats > 2 && g.anyStatGoad {
		// a static Goad$ True has no goader in the kernel (no resolving
		// controller owns it, so its marks have no seat to record): the
		// CR 701.15b defender gate in attackOffersN cannot read it, so a
		// larger game refuses such a table at its start (StallTwoSeat)
		g.twoSeatOnly = true
	}
}

// goadOn reports whether goad bookkeeping is live for k's table.
func goadOn(k *prep) bool { return k.gen != nil && k.gen.anyGoad }

// goadMech is seat p's goad mech (goadMechs: the arena marks, one per seat).
func goadMech(p uint8) mech { return goadMechs[p] }

// goadedAt reports whether the permanent at sl is goaded by anyone.
func (s *State) goadedAt(k *prep, sl Slot) bool {
	if DebugIgnoreGoad {
		return false
	}
	for sd := uint8(0); sd < MaxSeats; sd++ {
		if s.mSl(k, goadMech(sd), sl) {
			return true
		}
	}
	return k.gen.anyStatGoad && s.staticFlagGen(k, kop.StatGoad, sl) // a static's Goad$ True (kop.StatGoad)
}

// goad is OpGoad: the resolving controller goads Obj, or every battlefield
// permanent matching F (gorge effGoad over Defined$: battlefield objects
// only; a repeat by the same goader is one relationship).
func (s *State) goad(k *prep, in *kop.Instr) {
	p := s.res.e.ctrl
	if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
		// Defined$ Remembered is the whole set (gorge effGoad's Defined$
		// walk); objRef alone would goad only rem[0]. The overload
		// each-variant of Spectacular Showdown reaches this.
		for i := 0; i < int(s.g.nrem); i++ {
			o := s.g.rem[i]
			if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
				continue
			}
			s.goadSet(k, s.objs[o].slot, p)
		}
		return
	}
	if in.Obj == kop.RefNone {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:n] {
			s.goadSet(k, sl, p)
		}
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.goadSet(k, sl, p)
	}
}

func (s *State) goadSet(k *prep, sl Slot, p uint8) {
	if m := goadMech(p); !s.mSl(k, m, sl) {
		s.mSlSet(k, m, sl)
		s.touchSl(sl)
	}
}

// goadTurn ends the goads seat p made as p's turn begins (gorge
// expireTurnGoads at TurnChange).
func (s *State) goadTurn(k *prep, p uint8) {
	g := goadMech(p)
	for m := s.mSlMask(k, g); m.any(); m = m.rest() {
		s.touchSl(m.first())
	}
	s.mSlZero(k, g)
}

// goadLeave clears slot sl's goads as its permanent leaves.
func (s *State) goadLeave(k *prep, sl Slot) {
	for sd := uint8(0); sd < MaxSeats; sd++ {
		s.mSlClr(k, goadMech(sd), sl)
	}
}

// mustAttackGoad reports whether the able creature at sl must attack
// because it is goaded (the caller checked canAttack).
func (s *State) mustAttackGoad(k *prep, sl Slot) bool {
	return goadOn(k) && s.goadedAt(k, sl)
}

// execGoad runs this family's ops (execUnread's default).
//
//go:noinline
func (s *State) execGoad(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpGoad:
		s.goad(k, in)
		return true, true
	}
	return s.execInitiative(k, in) // act_initiative.go
}
