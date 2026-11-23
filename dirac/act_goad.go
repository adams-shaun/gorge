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
// (kop.StatGoad, anyStatGoad). A Goad$ True static an Animate staticAbilities$
// grant switches on is goaded by the seat that made the grant
// (grantGoader, goadedGranted).
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
}

// goadGrantBits is the OvGate bits of card c's granted StatGoad statics.
func goadGrantBits(c *kop.CardDef) (bits uint8) {
	for i := range c.Abilities {
		if a := &c.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatGoad {
			bits |= a.Static.OvGate
		}
	}
	return bits
}

// grantGoader is the value an override record stores in its (otherwise unused)
// t for a grant of a StatGoad static: the granting controller + 1, since
// gorge registers the granted static with the Animate's own controller
// (registerAnimateStaticAbilities: ce.Controller = c.Controller) and CR 701.15
// names the goader as the one the goad effect is controlled by. 0 for any
// other grant, so it still merges as before.
func grantGoader(k *prep, src CardID, o *kop.Override, ctrl uint8) int16 {
	if int(src) < len(k.t.Cards) && goadGrantBits(&k.t.Cards[src])&o.GrantStatics != 0 {
		return int16(ctrl) + 1
	}
	return 0
}

// goadedGranted reports whether seat p made a live override grant on sl of a
// Goad$ True static that applies to sl (gorge staticGoaders over the
// Animate's registered continuous effect, its controller the goader). The
// static's filter and condition resolve against the holder, as
// grantedStaticFlag's do.
func (s *State) goadedGranted(k *prep, sl Slot, p uint8) bool {
	if !s.fxM.has(sl) {
		return false
	}
	hit := false
	pm := &s.perms[sl]
	lts, lost := uint16(0), false
	known := false
	s.eachOvRec(sl, func(r ovRec) {
		if hit || r.idx == 0 || r.t != int16(p)+1 {
			return
		}
		o := &k.t.Overrides[r.idx-1]
		if o.GrantStatics == 0 {
			return
		}
		if !known {
			lts, lost = s.lossAt(k, sl)
			known = true
		}
		if lost && r.ts < lts {
			return
		}
		cd := &k.t.Cards[r.src]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			st := &a.Static
			if a.Kind != kop.AbStatic || st.Kind != kop.StatGoad || st.OvGate == 0 || o.GrantStatics&st.OvGate == 0 {
				continue
			}
			ctx := evalCtx{src: pm.obj, srcSl: sl, ctrl: pm.ctrl}
			if (st.Cond2 == 0 || s.condEval(k, st.Cond2, ctx)) && s.matchPermGen(k, st.Filter, sl, pm.ctrl, sl, layerFull) {
				hit = true
				return
			}
		}
	})
	return hit
}

// goadedBy reports whether seat p goads the permanent at sl (gorge
// combat.GoadedBy): an OpGoad mark p made, or a live printed Goad$ True
// static p controls (gorge staticGoaders: the static's controller is the
// goader; Psychic Impetus, Shiny Impetus).
func (s *State) goadedBy(k *prep, sl Slot, p uint8) bool {
	if s.mSl(k, goadMech(p), sl) {
		return true
	}
	if !k.gen.anyStatGoad || DebugStatGoadNoGoader {
		return false
	}
	if s.goadedGranted(k, sl, p) {
		return true
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if st.ctrl == p && a.Kind == kop.AbStatic && a.Static.Kind == kop.StatGoad && a.Static.OvGate == 0 && s.statCond(k, st) &&
			(st.emb == 0 || s.embCaptureOK(k, st.emb, sl)) &&
			s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// DebugStatGoadNoGoader is the pod lockstep mutation: a static goad names
// no goader, so attackOffersN offers its controller as a defender
// (TestFourSeatStaticGoadMutation must then diverge).
var DebugStatGoadNoGoader bool

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

// FeatGoadedBy reports whether seat p's goad designation is on battlefield
// object o (gorge view.CardView.GoadedBy: state's goads in force, not a
// Goad$ True static's, which is derived).
func (s *State) FeatGoadedBy(o Obj, p uint8) bool {
	k := s.k()
	ob := &s.objs[o]
	if DebugIgnoreGoad || !goadOn(k) || ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot {
		return false
	}
	return s.mSl(k, goadMech(p), ob.slot)
}

// FeatAttacksPlayer reports whether attack target code tgt names a player
// (not a planeswalker or a battle).
func FeatAttacksPlayer(tgt TCode) bool { return isPlayerTarget(tgt) }
