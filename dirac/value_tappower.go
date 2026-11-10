package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// KC9: sweep, five grammar sub-classes. Cold and out of line.

// DebugIgnoreTapPower is the graveexile mutation switch: tapPowerValue returns 0,
// so a Crew/Saddle payment that needs the TapPowerValue bonus cannot be
// offered. TestGraveExileAndTapPowerMutation requires the lockstep to diverge.
var DebugIgnoreTapPower bool

// DebugIgnoreGraveExile is the graveexile mutation switch: the dual-origin walk
// drops the Exile half (both the hidden pick and ChangeZoneAll).
// TestGraveExilePickMutation and TestGraveExileChangeZoneAllMutation (kernel
// scenarios) require each half to stop offering/moving the exiled card.
var DebugIgnoreGraveExile bool

// tapPowerValue is the candidate's Mode$ TapPowerValue bonus (graveexile,
// Tapestry Warden, Sulu): a Crew cost reads the crew-scoped statics, a Saddle
// cost the saddle-scoped ones, and a Station amount the station-scoped ones
// (station true, flags 0). The callers add the candidate's power, except for
// Value$ Toughness, which replaces it: this returns the difference from plain
// power. One static per ValidSA$ alternative: Filter matches the candidate,
// Filter2 the action's host permanent (Spacecraft on Sulu's station static), and
// the value is a literal A or Static.VA, evaluated with the candidate as its
// source (Sulu's Count$CardPower/Twice). gorge
// statics_assignment.go tapPowerValueFor.
//
//go:noinline
func (s *State) tapPowerValue(k *prep, sl Slot, flags kop.CostFlags, station bool, host Slot) int {
	if DebugIgnoreTapPower {
		return 0
	}
	base, added := s.tapPowerRead(k, sl, flags, station, host)
	p, _ := s.pt(k, sl)
	return base - p + added
}

// tapPowerTotal is the candidate's power plus every scoped TapPowerValue
// bonus: Station's PutCounter amount (CR 702.184a, at resolution). It shares
// tapPowerRead with tapPowerValue, so the offer, the payment and the effect
// cannot disagree.
//
//go:noinline
func (s *State) tapPowerTotal(k *prep, sl Slot, flags kop.CostFlags, station bool, host Slot) int {
	if DebugIgnoreTapPower {
		p, _ := s.pt(k, sl)
		return p
	}
	base, added := s.tapPowerRead(k, sl, flags, station, host)
	return base + added
}

// tapPowerRead returns the candidate's base (power, or toughness under a
// Toughness static) and the additive bonuses. The per-card TapPowerCrew /
// TapPowerSaddle constants remain for tables generated before StatTapPower;
// new tables emit only the static path.
func (s *State) tapPowerRead(k *prep, sl Slot, flags kop.CostFlags, station bool, host Slot) (base, added int) {
	p, t := s.pt(k, sl)
	base = p
	v := 0
	switch {
	case !station && flags&kop.CostCrew != 0:
		v = int(k.t.Cards[s.cardAt(sl)].TapPowerCrew)
	case !station && flags&kop.CostSaddle != 0:
		v = int(k.t.Cards[s.cardAt(sl)].TapPowerSaddle)
	}
	if v != 0 && s.tapPowerLost(k, sl) {
		// CR 613.1f: the bonus is the card's own static ability, so a
		// permanent that has lost all abilities (Flood the Engine on
		// Interface Ace) crews with its plain power (gorge tapPowerValueFor
		// walks only active statics).
		v = 0
	}
	switch {
	case v == int(kop.TapPowToughness):
		if !DebugToughAssignOff {
			base = t
		}
	case v != 0:
		added += v
	}
	if !k.anyTapPowerStatic {
		return base, added
	}
	visit := func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatTapPower {
			return true
		}
		if k.gen == nil {
			// no generated extension: a static with a gate or a condition
			// needs one, so it cannot hold here
			if a.Static.OvGate != 0 || a.Static.Cond2 != 0 {
				return true
			}
		} else if !s.statCond(k, st) {
			return true
		}
		if !tapPowerScopeHit(&a.Static, flags, station) {
			return true
		}
		// the qualifier names the action's own permanent; a gone host reads
		// every qualifier as met, as gorge's tapPowerValueFor does
		if a.Static.Filter2 != 0 && host != NoSlot &&
			!s.matchPerm(k, a.Static.Filter2, host, st.ctrl, st.sl) {
			return true
		}
		if !s.matchPerm(k, a.Static.Filter, sl, st.ctrl, st.sl) {
			return true
		}
		switch {
		case a.Static.StFlags&kop.StTapPowerTough != 0:
			if !DebugToughAssignOff {
				base = t
			}
		case a.Static.StFlags&kop.StDynamic != 0:
			added += s.evalV(k, a.Static.VA, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: st.ctrl})
		default:
			added += int(a.Static.A)
		}
		return true
	}
	if k.gen != nil {
		s.eachStatic(k, visit)
		return base, added
	}
	// a table with no generated extension: genLost and eachStatic read it, so
	// walk the statics directly (a lose-all override skips the permanent,
	// CR 613.1f, as tapPowerLost does)
	for m := s.statM; m.any(); m = m.rest() {
		ssl := m.first()
		if s.perms[ssl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		pp := &s.perms[ssl]
		cd := &k.t.Cards[s.cardAt(ssl)]
		for i := range cd.Abilities {
			visit(statSrc{a: &cd.Abilities[i], sl: ssl, ctrl: pp.ctrl, obj: pp.obj})
		}
	}
	return base, added
}

// tapPowerScopeHit reports whether st scopes the action being paid: a Station
// amount, a Crew cost or a Saddle cost.
func tapPowerScopeHit(st *kop.Static, flags kop.CostFlags, station bool) bool {
	switch {
	case station:
		return st.StFlags&kop.StTapPowerStation != 0
	case flags&kop.CostCrew != 0:
		return st.StFlags&kop.StTapPowerCrew != 0
	case flags&kop.CostSaddle != 0:
		return st.StFlags&kop.StTapPowerSaddle != 0
	}
	return false
}

// tapPowerAt is the ExTapPower value head: the tap-power total of the object
// Ref names, with the host the resolving Station ability's source. A tapped
// creature that has left reads its last known power and toughness (the
// vPowerOf read), and the station-scoped statics still in force read the card
// where it now is, its owner as its controller (gorge tapPowerValueFor over
// the departed object: Tapestry Warden's toughness for a Survey Mechan
// sacrificed before the station resolved).
func (s *State) tapPowerAt(k *prep, r kop.Ref, ctx evalCtx) int {
	if sl := s.tapPowerSlot(r, ctx); sl != NoSlot {
		return s.tapPowerTotal(k, sl, 0, true, ctx.srcSl)
	}
	pw, t := s.refPT(k, r, ctx)
	if DebugIgnoreTapPower || !k.anyTapPowerStatic || k.gen == nil || r != kop.RefTriggerObj || ctx.e == nil || ctx.e.trigObj == NoObj {
		return pw
	}
	o := ctx.e.trigObj
	ob := &s.objs[o]
	base, added := pw, 0
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatTapPower || !s.statCond(k, st) || !tapPowerScopeHit(&a.Static, 0, true) {
			return true
		}
		if a.Static.Filter2 != 0 && ctx.srcSl != NoSlot && !s.matchPerm(k, a.Static.Filter2, ctx.srcSl, st.ctrl, st.sl) {
			return true
		}
		if !s.matchCardGen(k, a.Static.Filter, ob.card, ob.owner(), st.ctrl, o) {
			return true
		}
		switch {
		case a.Static.StFlags&kop.StTapPowerTough != 0:
			if !DebugToughAssignOff {
				base = t
			}
		case a.Static.StFlags&kop.StDynamic != 0:
			added += s.evalV(k, a.Static.VA, evalCtx{src: o, srcSl: NoSlot, ctrl: st.ctrl})
		default:
			added += int(a.Static.A)
		}
		return true
	})
	return base + added
}

// tapPowerSlot is the battlefield slot the tap-power ref names, live as the
// same object: the triggering object for Station's picked creature, else the
// object the ref resolves to.
func (s *State) tapPowerSlot(r kop.Ref, ctx evalCtx) Slot {
	if r == kop.RefTriggerObj {
		e := ctx.e
		if e == nil || e.trigObj == NoObj {
			return NoSlot
		}
		ver := s.objs[e.trigObj].ver
		if e.li != 0 {
			ver = s.lk(e.li).ver
		}
		return s.slotIfSame(e.trigObj, ver)
	}
	o := s.refObjCtx(r, ctx)
	if o == NoObj {
		return NoSlot
	}
	return s.slotIfSame(o, s.objs[o].ver)
}

// graveExileCands adds a Graveyard,Exile hidden pick's candidates (graveexile):
// gorge's gameWide pool walks the origin zones in order (Graveyard, then
// Exile), each player in seat order, and matches ChangeType$ against the
// card. The fetch player is the controller, so an Owner$ clause reads the
// card's real owner against it.
//
//go:noinline
func (s *State) graveExileCands(k *prep, in *kop.Instr, d *Decision) {
	ctrl := s.res.e.ctrl
	nz := 2
	if DebugIgnoreGraveExile {
		nz = 1
	}
	for zi := 0; zi < nz; zi++ {
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			pl := &s.pl[p]
			var list []Obj
			if zi == 0 {
				list = pl.grave[:pl.graveN]
			} else {
				list = pl.exile[:pl.exileN]
			}
			for _, o := range list {
				if s.picksHas(o) {
					continue
				}
				if s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), ctrl, o) {
					d.add(MakeAction(AkChoose, uint8(o), 0, 0))
				}
			}
		}
	}
}

// tapPowerLost reports whether the permanent at sl has lost all abilities,
// for tapPowerValue: an override, a one-shot loss, an attached Aura's
// lose-all static, or a face-down face.
func (s *State) tapPowerLost(k *prep, sl Slot) bool {
	if s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return true
	}
	if k.gen == nil {
		return false
	}
	if _, lost := s.lossAt(k, sl); lost {
		return true
	}
	return s.genLost(k, sl)
}
