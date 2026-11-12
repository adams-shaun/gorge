package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ops: cold arms, reached only from
// execGen's default branch. done=false means the op is not one of these.
//
//go:noinline
func (s *State) execCold1(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpPutCounterAll:
		s.putCounterAll(k, in)
	case kop.OpUntapAll:
		s.untapAll(k, in)
	case kop.OpDealDamageFrom:
		// gorge DealDamage with DamageSource$ (Polliwallop: the creature
		// deals twice its power): as OpBite, both must still be creatures
		// on the battlefield; the source creature is the damage source. A
		// planeswalker is a source too (Compel Brutality's second mode:
		// loyalty damage, DamageSource$ ParentTarget)
		if in.B.N == 1 {
			s.dmgSrcPlayers(k, in) // players mode (act_damagefrom.go)
			break
		}
		a := s.objRef(k, in.Obj)
		b := s.objRef(k, kop.Ref(in.C))
		if a != NoSlot && b != NoSlot && (s.isCreature(k, a) || s.typesAt(k, a)&kop.TPlaneswalker != 0) && (s.isCreature(k, b) || s.typesAt(k, b)&kop.TPlaneswalker != 0) {
			n := s.value(k, in.A)
			if DebugDmgAmountPlusOne {
				n++
			}
			s.damageBetween(k, a, b, n)
		}
	default:
		return s.execCold2(k, in)
	}
	return true, true
}

// execCold2 runs the ops (reached from execCold1's default branch, cold).
// done=false means the op is not one of these.
//
//go:noinline
func (s *State) execCold2(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpGainEnergy:
		s.gainEnergy(k, in)
	case kop.OpOverrideAll:
		if in.Obj != kop.RefNone {
			s.overrideAllScoped(k, in)
		} else {
			s.overrideAll(k, in)
		}
	case kop.OpRollDie:
		s.rollDie(k, in)
	default:
		return s.execCold3(k, in)
	}
	return true, true
}

// Ops: cold arms reached
// only from execCold2's default branch. done=false means the op is not one
// of these.
//
//go:noinline
func (s *State) execCold3(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpTapAll:
		s.tapAll(k, in)
	case kop.OpSeek:
		s.seek(k, in)
	case kop.OpAmass:
		s.amass(k, in)
	case kop.OpConnive:
		return s.connive(k, in), true
	case kop.OpConniveCounters:
		s.conniveCounters(k, in)
	default:
		return s.execCold4(k, in)
	}
	return true, true
}

// askOptionsCold1 builds the resolution asks' options (resolveOptionsGen's
// default); false: not one of them.
//
//go:noinline
func (s *State) askOptionsCold1(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenConfirm:
		d.Sub = ChGenConfirm
		d.add(MakeAction(AkChoose, 0, 0, 1)) // gorge's options: yes, no
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raVote:
		s.voteOptions(k, d) // act_vote.go
	case raDrawUpto:
		s.drawUptoOptions(d) // act_drawupto.go
	case raGenDestAlt:
		s.destAltOptions(d) // zone_destalt.go
	default:
		return s.askOptionsCold2(k, d)
	}
	return true
}

// applyAskCold1 applies an answer to a resolution ask (applyResolveGen's
// default); false: not one of them.
//
//go:noinline
func (s *State) applyAskCold1(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenConfirm:
		s.res.ask = raNone
		if a.Mode() == 0 {
			s.continueRes(k, 0) // declined: the owner is skipped
			return true
		}
		if s.searchPick(k) && !s.moveAsked() {
			s.continueRes(k, 0)
		}
	case raVote:
		s.voteAnswer(k, a) // act_vote.go
	case raDrawUpto:
		s.drawUptoAnswer(k, a) // act_drawupto.go
	case raGenDestAlt:
		s.destAltAnswer(k, a) // zone_destalt.go
	default:
		return s.applyAskCold2(k, a)
	}
	return true
}

// execCold4 runs the ops (cold; reached from execCold3's default;
// its own default chains to execCold5).
// done=false means the op is not one of these.
//
//go:noinline
func (s *State) execCold4(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpAtEOT:
		if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.atEOTRider(s.perms[sl].obj, in.C == 1)
		}
	case kop.OpSkipNextUntap:
		if in.F != 0 {
			s.skipNextUntapAll(k, in) // cast_altfree.go
		} else if sl := s.objRef(k, in.Obj); sl != NoSlot {
			s.touchSl(sl)
			s.perms[sl].flags |= pfSkipUntap
		}
	default:
		return s.execCold5(k, in)
	}
	return true, true
}

// execCold5 runs the ops (reached from execCold3's default branch, cold).
// done=false means the op is not one of these.
//
//go:noinline
func (s *State) execCold5(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpAttachObj:
		s.attachObj(k, in)
	case kop.OpTapOrUntap:
		return s.tapOrUntap(k, in), true
	case kop.OpExplore:
		return s.explore(k, in), true
	case kop.OpSetLife:
		// the amount is read once, before any change (gorge
		// NumResolvedStrict ahead of the acting-player walk)
		q := s.players(k, in.Obj)
		n := s.value(k, in.A)
		for i := uint8(0); i < q.n; i++ {
			p := q.s[i]
			if d := n - int(s.pl[p].life); d > 0 {
				s.gainLife(k, p, d)
			} else {
				s.loseLife(k, p, -d)
			}
		}
	case kop.OpSkipUntap:
		s.skipUntap(k, in)
	case kop.OpDiscover:
		s.discover(k, in)
	case kop.OpMoveCounter:
		s.moveCounter(k, in)
	case kop.OpRearrange:
		return s.rearrange(k, in), true
	case kop.OpFog:
		s.g.fog = true
	case kop.OpPoison:
		q := s.players(k, in.Obj)
		n := s.value(k, in.A)
		for i := uint8(0); i < q.n; i++ {
			s.addPoison(q.s[i], n)
		}
	default:
		return s.execCold6(k, in)
	}
	return true, true
}

// askOptionsCold2 lists a resolution ask's options (askOptionsCold1's
// default); false: not one of them.
//
//go:noinline
func (s *State) askOptionsCold2(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenTapOrUntap:
		// gorge's options: tap, untap -- the untap first on a tapped
		// permanent (the state-changing choice leads)
		d.Sub = ChGenTapOrUntap
		in := &s.prog(k)[s.res.pc]
		sl := s.objRef(k, in.Obj)
		o := uint8(s.perms[sl].obj)
		first, second := uint8(1), uint8(0)
		if s.perms[sl].flags&pfTapped != 0 {
			first, second = 0, 1
		}
		d.add(MakeAction(AkChoose, o, 0, first))
		d.add(MakeAction(AkChoose, o, 0, second))
	case raGenRearrange:
		// gorge's KArrange over the looked-at cards: each pick is the next
		// card from the top; every card is ordered (no Done)
		d.Sub = ChArrange
		r := &s.res
		for i := 0; i < int(r.peekN); i++ {
			if r.peekSel&(1<<i) == 0 {
				d.add(MakeAction(AkChoose, uint8(r.peek[i]), 0, 0))
			}
		}
	case raGenRearrShuffle:
		d.Sub = ChGenConfirm // gorge's options: yes, no
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenDemonstrate:
		// gorge's demonstrate election: yes (copy it), no
		d.Sub = ChGenConfirm
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenExplore:
		// gorge's options: the graveyard (option 0), back on top
		d.Sub = ChGenExplore
		pl := &s.pl[s.res.askP]
		top := uint8(pl.lib[pl.libLo])
		d.add(MakeAction(AkChoose, top, 0, 0))
		d.add(MakeAction(AkChoose, top, 0, 1))
	default:
		return s.askOptionsCold3(k, d)
	}
	return true
}

// applyAskCold2 applies an answer to a resolution ask (applyAskCold1's
// default); false: not one of them.
//
//go:noinline
func (s *State) applyAskCold2(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenTapOrUntap:
		s.res.ask = raNone
		in := s.prog(k)[s.res.pc]
		in.Op = kop.OpUntap // gorge TryUntap: a stun counter is removed instead
		if a.Mode() == 1 {
			in.Op = kop.OpTap // gorge EmitTap (Taps triggers)
		}
		s.exec(k, &in)
		s.continueRes(k, 0)
	case raGenRearrange:
		r := &s.res
		for i := 0; i < int(r.peekN); i++ {
			if r.peek[i] == Obj(a.Source()) {
				r.peekSel |= 1 << i
			}
		}
		r.arr[r.peekI] = Obj(a.Source())
		r.peekI++
		if r.peekI < r.peekN {
			return true
		}
		if s.rearrangeDone(k) {
			s.continueRes(k, 0)
		}
	case raGenRearrShuffle:
		s.res.ask = raNone
		if a.Mode() == 1 {
			if p := s.playerRef(k, s.prog(k)[s.res.pc].Obj); p < MaxSeats {
				s.shuffleLib(p)
			}
		}
		s.continueRes(k, 0)
	case raGenExplore:
		s.res.ask = raNone
		s.exploreNonland(k, &s.prog(k)[s.res.pc], a.Mode() == 0)
		s.continueRes(k, 0)
	case raGenDemonstrate:
		s.res.ask = raNone
		if a.Mode() == 0 {
			s.continueRes(k, 0)
			break
		}
		if s.demonstrateCopy(k, demoStart) {
			s.continueRes(k, 0)
		}
	default:
		return s.applyAskCold3(k, a)
	}
	return true
}

// execCold6 runs these ops (execCold5's default branch, cold). done=false
// means the op is not one of these.
//
//go:noinline
func (s *State) execCold6(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpBolster:
		return s.bolster(k, in), true
	}
	return s.execCrewSaddle(k, in)
}

// askOptionsCold3 builds these resolution asks' options; false: not one.
//
//go:noinline
func (s *State) askOptionsCold3(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenBolster:
		d.Sub = ChGenBolster
		var buf [MaxPerm]Slot
		n := s.bolsterCands(k, s.res.askP, &buf)
		for _, sl := range buf[:n] {
			d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
		}
	case raGenProliferate:
		d.Sub = ChGenProlif
		s.prolifOptions(k, d)
	case raGenEmpower:
		d.Sub = ChGenBolster
		in := &s.prog(k)[s.res.pc]
		var buf [MaxPerm]Slot
		n := s.empowerCands(k, s.res.askP, s.empowerSub(k, in), buf[:])
		for _, sl := range buf[:n] {
			d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
		}
	default:
		return s.askOptionsCold4(k, d)
	}
	return true
}

// applyAskCold3 applies an answer to one of these resolution asks; false: not one.
//
//go:noinline
func (s *State) applyAskCold3(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenBolster:
		s.res.ask = raNone
		in := &s.prog(k)[s.res.pc]
		if ob := &s.objs[a.Source()]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
			s.bolsterPut(k, ob.slot, int(in.C), s.value(k, in.A))
		}
		s.continueRes(k, 0)
	case raGenProliferate:
		s.answerProlif(k, a)
	case raGenEmpower:
		s.res.ask = raNone
		in := &s.prog(k)[s.res.pc]
		if ob := &s.objs[a.Source()]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && s.perms[ob.slot].ctrl == s.res.askP {
			if n := s.value(k, in.A); n > 0 {
				s.bolsterPut(k, ob.slot, int(kop.CtrLoyalty), n)
			}
		}
		s.continueRes(k, 0)
	default:
		return s.applyAskCold4(k, a)
	}
	return true
}

// execCold7 runs the ops (execCrewSaddle's default branch, cold). done=false
// means the op is not one of these.
//
//go:noinline
func (s *State) execCold7(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpSupport:
		return s.support(k, in), true
	case kop.OpChangeSpeed:
		s.changeSpeedOp(k, in) // kw_speed.go
		return true, true
	}
	return s.execOblige(k, in) // act_mayshuffle.go: OpMayShuffleMove
}

// askOptionsCold4 builds the resolution asks' options; false: not one.
//
//go:noinline
func (s *State) askOptionsCold4(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenSupport:
		d.Sub = ChGenBolster // gorge's "counter_pick"
		s.supportOptions(k, &s.prog(k)[s.res.pc], d)
	default:
		return s.askOptionsOblige(k, d)
	}
	return true
}

// applyAskCold4 applies an answer to a resolution ask; false: not one.
//
//go:noinline
func (s *State) applyAskCold4(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenSupport:
		s.answerSupport(k, a)
	default:
		return s.applyAskOblige(k, a)
	}
	return true
}

// execEachFromSource runs these ops (execCrewSaddle's default branch, cold). done=false
// means the op is not one of these.
//
//go:noinline
func (s *State) execEachFromSource(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpPutCountersFrom:
		s.putCountersFrom(k, in)
		return true, true
	case kop.OpClearCounters:
		s.clearCounters(k, in)
		return true, true
	case kop.OpRemoveCounterAll:
		s.removeCounterAll(k, in)
		return true, true
	case kop.OpMonstrosity:
		s.monstrosity(k, in) // act_monstrosity.go
		return true, true
	case kop.OpEmpower:
		if k.gen.anyEmpower { // never on FDN/BLB (act_empower.go prepEmpower)
			return s.empower(k, in), true // act_empower.go
		}
	}
	return s.execCold7(k, in) // act_support.go: OpSupport
}

// execUnread runs this family's ops (execMcombat's default); done=false
// means the op is not one of them.
//
//go:noinline
func (s *State) execUnread(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpMayAsk:
		return s.mayAsk(k, in), true
	case kop.OpToLibraryPair:
		s.toLibraryPair(k, in)
		return true, true
	case kop.OpDiscardUnless:
		return s.discardUnless(k, in), true
	case kop.OpUntapChoose:
		return s.untapChoose(k, in), true
	case kop.OpLookAck:
		if p := s.playerRef(k, in.Obj); p < MaxSeats && !DebugNoLookAck && (in.C == 0 && s.pl[p].handN > 0 || in.C == 1 && s.value(k, in.A) > 0 && s.pl[p].libLen() > 0) {
			s.res.ask, s.res.askP = raGenLookAck, s.res.e.ctrl
			return false, true
		}
		return true, true
	case kop.OpBlight:
		return s.execBlight(k, in), true
	}
	return s.execGoad(k, in) // act_goad.go
}

// askOptionsUnread builds this family's resolution asks' options (the
// tail of askOptionsModalAct); false: not one.
//
//go:noinline
func (s *State) askOptionsUnread(k *prep, d *Decision) bool {
	switch s.res.ask {
	case raGenMayAsk:
		d.Sub = ChGenConfirm // gorge's options: yes, no
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenPeekReveal:
		s.askOptionsPeekReveal(d) // act_peekreveal.go
	case raGenDiscUnless:
		d.Sub = ChGenDiscUnless // gorge's options: unless, ordinary
		d.add(MakeAction(AkChoose, 0, 0, 1))
		d.add(MakeAction(AkChoose, 0, 0, 0))
	case raGenUntapPick:
		d.Sub = ChGenUntapPick
		var buf [MaxPerm]Slot
		n := s.untapChooseCands(k, &s.prog(k)[s.res.pc], &buf)
		for _, sl := range buf[:n] {
			d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
		}
		if s.prog(k)[s.res.pc].C == 1 {
			d.add(MakeAction(AkDone, 0, 0, 0))
		}
	case raGenDiscUnlessPick:
		d.Sub = ChDiscard // gorge's KModes "discard" over the matching cards, hand order
		in := &s.prog(k)[s.res.pc]
		pl := &s.pl[s.res.chosen]
		for i := 0; i < int(pl.handN); i++ {
			if o := pl.hand[i]; s.discardMatch(k, in.F, o) {
				d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			}
		}
	default:
		if s.askOptionsWinTap(k, d) { // mana_windowtapx.go
			return true
		}
		if s.askOptionsBlight(k, d) { // act_blight.go
			return true
		}
		if s.askOptionsTurnUpEff(k, d) { // face_turnupeffect.go
			return true
		}
		if s.askOptionsDistPick(k, d) { // ctr_distpick.go
			return true
		}
		if s.askOptionsPauperkw(k, d) { // kw_madness.go
			return true
		}
		if s.askOptionsExtort(k, d) { // act_extort.go
			return true
		}
		return s.askOptionsEvidence(k, d) // kw_wardcost.go
	}
	return true
}

// applyAskUnread applies an answer to this family's resolution asks (the
// tail of applyAskModalAct); false: not one.
//
//go:noinline
func (s *State) applyAskUnread(k *prep, a Action) bool {
	switch s.res.ask {
	case raGenMayAsk:
		skip := uint8(0)
		if a.Mode() == 0 && !s.topLibForceMove(k) {
			skip = uint8(s.value(k, s.prog(k)[s.res.pc].A))
		}
		s.continueRes(k, skip)
	case raGenPeekReveal:
		s.res.ask = raNone
		s.applyAskPeekReveal(k, a) // act_peekreveal.go
	case raGenDiscUnless:
		s.res.ask = raNone
		if a.Mode() == 0 {
			if s.discardOrdinary(k, &s.prog(k)[s.res.pc], s.res.chosen) {
				s.continueRes(k, 0)
			}
			break
		}
		if o, n := s.unlessMatch(k, &s.prog(k)[s.res.pc], s.res.chosen); n == 1 {
			s.discardObj(k, o)
		} else if n > 1 {
			s.res.ask = raGenDiscUnlessPick
			break
		}
		s.continueRes(k, 0)
	case raGenDiscUnlessPick:
		s.res.ask = raNone
		s.discardObj(k, Obj(a.Source()))
		s.continueRes(k, 0)
	case raGenUntapPick:
		r := &s.res
		if a.Kind() != AkDone && r.peekN < uint8(len(r.peek)) {
			r.peek[r.peekN] = Obj(a.Source())
			r.peekN++
			r.left--
			var buf [MaxPerm]Slot
			if r.left > 0 && s.untapChooseCands(k, &s.prog(k)[r.pc], &buf) > 0 {
				return true
			}
		}
		r.ask = raNone
		for i := 0; i < int(r.peekN); i++ {
			if ob := &s.objs[r.peek[i]]; ob.zone == kop.ZoneBattlefield {
				s.untapSlot(k, ob.slot)
			}
		}
		r.peekN = 0
		s.continueRes(k, 0)
	default:
		if s.applyAskWinTap(k, a) { // mana_windowtapx.go
			return true
		}
		if s.applyAskBlight(k, a) { // act_blight.go
			return true
		}
		if s.applyAskTurnUpEff(k, a) { // face_turnupeffect.go
			return true
		}
		if s.applyAskDistPick(k, a) { // ctr_distpick.go
			return true
		}
		if s.applyAskPauperkw(k, a) { // kw_madness.go
			return true
		}
		if s.applyAskExtort(k, a) { // act_extort.go
			return true
		}
		return s.applyAskEvidence(k, a) // kw_wardcost.go
	}
	return true
}

// askOptionsPauperkw builds this file's resolution asks' options (the tail
// of askOptionsUnread); false: not one.
//
//go:noinline
func (s *State) askOptionsPauperkw(k *prep, d *Decision) bool {
	if s.res.ask == raGenMiracle {
		return s.miracleAskOptions(k, d) // kw_miracle.go
	}
	if s.res.ask != raGenMadness {
		return false
	}
	d.Sub = ChOptional
	o := s.res.e.obj
	if s.madnessCastable(k, o, s.res.askP) {
		d.add(MakeAction(AkChoose, 0, 0, 1))
	}
	d.add(MakeAction(AkChoose, 0, 0, 0))
	return true
}

// applyAskPauperkw applies an answer to this file's resolution asks (the
// tail of applyAskUnread); false: not one.
//
//go:noinline
func (s *State) applyAskPauperkw(k *prep, a Action) bool {
	if s.res.ask == raGenMiracle {
		return s.applyMiracleAsk(k, a) // kw_miracle.go
	}
	if s.res.ask != raGenMadness {
		return false
	}
	s.res.ask = raNone
	o, p := s.res.e.obj, s.res.e.ctrl
	s.finishResolve(k, false)
	if a.Mode() == 1 && s.objs[o].zone == kop.ZoneExile && s.madnessCastable(k, o, p) {
		// the caster (beginCast reads it); after the cast the priority
		// holder the resolution left (the active player, CR 117.3b) gets
		// priority, as gorge resumes after the madness trigger
		holder := s.prio
		s.prio = p
		s.beginCast(k, o, 0)
		s.prio = holder
		s.bld().e.trigPl = trigPlMadness
		return true
	}
	if s.objs[o].zone == kop.ZoneExile {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneGraveyard, false, from, false) // gorge madnessDeclined
	}
	return true
}

// execStdmech runs this family's ops (execInitiative's default).
//
//go:noinline
func (s *State) execStdmech(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpUnearth:
		s.unearth(k)
		return true, true
	case kop.OpExt:
		return s.execExt(k, in) // extop.go
	}
	return false, false
}

func (s *State) askOptionsStdmech(k *prep, d *Decision) bool {
	if s.res.ask != raPrevColor {
		return s.askOptionsRca(k, d) // ctr_removeany.go
	}
	d.Sub = ChGenColor
	for c := uint8(0); c < 5; c++ {
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
	return true
}

// applyAskStdmech records the chosen colour as a new effect record. Each
// effect keeps the colour chosen as it was created
// (cr-effect-chosencolor-snapshot): the same card resolving again (a
// flashback cast) adds a record of the new colour beside the earlier one;
// each effect is its own replacement candidate (two effects of one card
// are ordered).
func (s *State) applyAskStdmech(k *prep, a Action) bool {
	if s.res.ask != raPrevColor {
		return s.applyAskRca(k, a) // ctr_removeany.go
	}
	o, col := s.res.e.obj, kop.ColorMask(1)<<a.Mode()
	free := -1
	for i := range s.g.prevCol {
		r := &s.g.prevCol[i]
		if DebugPrevColRecolor && r.col != 0 && r.obj == o {
			r.col = col // the pre-690ed17f0 reading: the card's live ChosenColor
		} else if r.col == 0 && r.sc == 0 && free < 0 {
			free = i
		}
	}
	if free < 0 {
		s.capFull(StallPrevCol) // loud: never drop an effect
	} else {
		s.g.prevCol[free].obj, s.g.prevCol[free].col = o, col
	}
	s.continueRes(k, 0)
	return true
}
