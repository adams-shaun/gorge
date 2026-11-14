package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Turn structure follows gorge's engine/rules/turn.go: untap and cleanup
// pose no priority; every other step runs a priority round (active player
// first) even when nothing can happen; the starting player skips the turn-1
// draw step; with no attackers the game jumps from declare attackers to end
// of combat keeping the priority holder (non-active player, one pass
// already counted), and after regular combat damage it jumps to end of
// combat with no priority round in the damage step.

func (s *State) beginTurn(p uint8) {
	if s.turn == 0 && s.leylineStart(s.k()) {
		return
	}
	if s.pmOn != 0 {
		s.persistTurn() // the PersistentMana$ tally ends with the turn, its units with the cleanup-to-untap boundary (mana_persistent.go)
	}
	if s.k().anyYourTurns && s.g.yourTurns[SeatIx(p)] < 255 {
		s.g.yourTurns[SeatIx(p)]++ // ExYourTurns: this turn is begun with p active
	}
	if s.k().anyRedNC {
		s.g.redNC = [MaxSeats]uint8{} // ExRedNCDmg: a fresh turn's tally
	}
	if s.k().anyDayNight {
		s.dnTurn() // CR 731.2's count, before the bank resets (kw_daynight.go)
	}
	if k := s.k(); k.gen != nil && k.gen.anyAtkLast {
		s.atkLastTurnEnd() // the outgoing seat's attacks become its last turn's (trig_atklastturn.go)
	}
	s.newTurnGen()
	if k := s.k(); k.gen != nil && k.gen.anySaddle {
		s.unsaddleAll(k)
	}
	if k := s.k(); k.gen != nil && k.gen.anySrcMark {
		s.srcMarksTurn() // filter_combatwords.go
	}
	if s.k().speed {
		s.speedTurn() // kw_speed.go
	}
	if goadOn(s.k()) {
		s.goadTurn(s.k(), p) // act_goad.go
	}
	s.turn++
	s.active = p
	if k := s.k(); k.gen != nil && k.gen.anySkipPhase {
		s.skipPhaseTurn(p) // act_skipphase.go
	}
	if k := s.k(); k.gen != nil && k.gen.anyMonarch {
		s.monarchTurn() // the turn-start monarch (act_monarch.go)
	}
	if k := s.k(); k.gen != nil && k.gen.anyNTEndEmb {
		s.embTurnStart(k, p) // UntilTheEndOfYourNextTurn: this turn is its last (static_emblemdurations.go)
	}
	s.step = kop.StepUntap
	s.ctl = ctlStepBegin
	s.carry = carryNone
	s.g.declM = slotMask{}
	live := s.occ
	if s.k().phasing {
		live = live.or(s.phasedSlots()) // a phased-out permanent's turn limits end too (CR 602.5b, 702.26d)
	}
	for m := live; m.any(); m = m.rest() {
		pm := &s.perms[m.first()]
		if pm.actUsed != 0 || pm.flags&pfLoyaltyUsed != 0 {
			s.touch(pm.obj)
		}
		pm.actUsed = 0
		pm.flags &^= pfLoyaltyUsed
	}
	for i := 0; i < int(s.nobj); i++ {
		if s.objs[i].activations() != 0 {
			s.touch(Obj(i))
			s.objs[i].acts &^= objActsMask
		}
	}
	for i := range s.pl {
		pl := &s.pl[i]
		if uint8(i) == SeatIx(p) {
			// gorge resets a seat's Player.LandsPlayed only when its own turn begins
			// (events foldTurnChange), so another seat's count outlives the turn it
			// was made in: the Condition-split gate reads it (mana_condsplit.go); the
			// projections show the active seat's only (landsProj)
			pl.landsPlayed = 0
		}
		pl.drawnTurn = 0
		pl.lifeGained = 0
		pl.lastLost = pl.lifeLost // ExLifeLostLastTurn: the turn just ended
		pl.lifeLost = 0
		if !DebugSacrificedNoTurnReset {
			food := pl.sac[sacFood]
			pl.sac = [3]uint8{}
			if DebugTurnTalliesNoReset {
				pl.sac[sacFood] = food // value_turntallies.go mutation hook
			}
		}
		if !DebugScryTallyNoTurnReset {
			pl.scry = 0
		}
		if !DebugDiscardedNoTurnReset {
			pl.disc = 0
		}
	}
	s.dmgTakenNewTurn() // gorge DamageTakenThisTurn folds back to the last TurnChange (kw_bloodthirst.go)
}

// openPriority starts the step's priority round.
func (s *State) openPriority() {
	switch s.carry {
	case carryAP:
		s.prio = s.active
		s.passes = 1
	default:
		s.prio = s.active
		s.passes = 0
	}
	s.carry = carryNone
	s.ctl = ctlPriority
}

// priority round openings (State.carry): gorge's setStep jumps keep the
// priority holder and pass count instead of starting a fresh round.
const (
	carryNone uint8 = iota
	carryAP         // the active player holds, one pass counted
)

// beginStep performs s.step's turn-based actions.
func (s *State) beginStep(k *prep) {
	switch s.step {
	case kop.StepUntap:
		if k.gen != nil && k.gen.ix[mechEchoOwed] != arenaNone && !s.g.untapOn {
			s.echoSnap(k) // before the untap clears the sickness (trig_echo.go)
		}
		if k.phasing && !s.g.untapOn {
			s.phaseUntap(k) // CR 702.26a/p, before the untap action (kw_phasing.go)
		}
		if k.anyDayNight && s.g.dayNight&3 != 0 && !s.g.untapOn {
			s.dnUntap(k) // CR 731.2, before the untap action (kw_daynight.go)
		}
		if k.gen != nil && (k.gen.untapRepl || k.gen.anySkipUntap) {
			s.untapStepGen(k) // untap replacements of other permanents (act_energy.go)
			return
		}
		pre := s.tappedMask(s.active)
		m := s.ctrlM[s.active]
		for m.any() {
			sl := m.first()
			m = m.rest()
			p := &s.perms[sl]
			f := p.flags
			p.flags &^= pfSick
			if p.flags&pfSkipUntap != 0 {
				p.flags &^= pfSkipUntap // CR 611.2b one-shot, consumed even untapped (gorge CantUntapNextStep)
				s.touchSl(sl)
				continue
			}
			blocked := k.cards[s.cardAt(sl)].noUntap && !(k.gen != nil && s.genLost(k, sl)) // a lost "doesn't untap" (Witness Protection)
			if p.flags&pfTapped != 0 && !blocked && p.ctrLow(int(kop.CtrStun)) > 0 {
				p.seeCtr(s)
				p.subCtrN(s, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d)
				s.touchSl(sl)
				continue
			}
			if p.flags&pfTapped != 0 && !blocked {
				p.flags &^= pfTapped
				s.untapEnds(k, p.obj) // the lock and the host-tapped grants end at the first untap (static_animatewhiletapped.go)
			}
			if p.flags != f {
				s.touchSl(sl)
			}
		}
		if k.gen != nil {
			s.fireUntapsGen(k, pre)
		}
		if k.gen != nil && k.gen.anyUntapOther {
			s.untapOthers(k)
		}
		s.step = kop.StepUpkeep
	case kop.StepUpkeep:
		if k.gen != nil && k.gen.anySuspend {
			s.suspendTick(k) // CR 702.62a: gorge ticks as the step is entered (kw_suspend.go)
		}
		s.firePhase(k, kop.StepUpkeep)
		if k.gen != nil && k.gen.anyDelayUp {
			s.fireDelayedUpkeep(k)
		}
		if s.ndly != 0 {
			s.fireDlyUpkeep(k) // AtEOT$ SacrificeUpkeep (trig_upkeepsac.go)
		}
		if initOn(k) {
			s.initUpkeep(k) // CR 726.2 (act_initiative.go)
		}
		s.openPriority()
	case kop.StepDraw:
		if s.turn == 1 && s.nSeats() == 2 {
			// CR 103.8a skips the starting player's first draw step in a
			// two-player game only; CR 103.8c: no one skips it at more
			s.step = kop.StepMain1
			return
		}
		s.g.drawFirst = true  // repl_draw.go: the first card drawn in this draw step
		s.g.drawStepN = 0     // trig_drawstep.go: no card drawn in this draw step yet
		if !s.out(s.active) { // a departed active player draws nothing (gorge turn.go)
			s.draw(s.active)
		}
		s.g.drawFirst = false
		if s.over {
			return
		}
		s.firePhase(k, kop.StepDraw)
		s.openPriority()
	case kop.StepMain1, kop.StepMain2:
		s.firePhase(k, s.step)
		if k.anySaga && s.step == kop.StepMain1 {
			s.advanceSagas(k) // CR 505.4 / 714.3c
		}
		s.openPriority()
	case kop.StepBeginCombat:
		s.firePhase(k, kop.StepBeginCombat)
		if k.gen != nil && k.gen.anyDelayUp {
			s.fireDelayedBeginCombat(k) // trig_delaybegincombat.go
		}
		s.openPriority()
	case kop.StepAttackers:
		s.nAttackers = 0
		s.declared = false
		s.lastPick = NoSlot
		s.declareMustAttack(k)
		if s.anyAttacker(k) {
			s.ctl = ctlAttackers
		} else if s.nAttackers > 0 {
			s.finishAttacks(k)
		} else {
			s.openPriority()
		}
	case kop.StepBlockers:
		s.lastPick = NoSlot
		s.blockSeq = 0
		if MaxSeats > 2 {
			s.blockRound(k, s.active)
		} else if s.anyBlock(k) {
			s.ctl = ctlBlockers
		} else {
			s.finishBlocks(k)
		}
	case kop.StepFirstStrikeDamage:
		s.damageStep(k, true, false)
	case kop.StepDamage:
		s.damageStep(k, false, false)
	case kop.StepEndCombat:
		s.firePhase(k, kop.StepEndCombat) // "at end of combat" (no FDN card yet)
		if k.gen != nil && k.gen.anyDelayUp {
			s.fireDelayedEndCombat(k)
		}
		if k.gen != nil && k.gen.anyTokEndCombat {
			s.fireDlyEndCombat(k) // after the dp programs: one registration-ordered list in gorge
		}
		s.openPriority()
	case kop.StepEnd:
		m := int(s.npend)
		s.firePhase(k, kop.StepEnd)
		s.fireDelayedAt(k, m)
		if k.gen != nil {
			s.fireDelayedGen(k)
			if k.gen.anyMonarch {
				s.monarchEndStep() // CR 725.2 (act_monarch.go)
			}
		}
		s.openPriority()
	case kop.StepCleanup:
		pl := &s.pl[s.active]
		if pl.handN > 7 || (k.gen != nil && k.gen.anyHandSize && pl.handN > 0) { // a numeric maximum can be below seven (static_playerstats.go)
			if max := s.maxHand(k, s.active); int(pl.handN) > max {
				s.discardLeft = pl.handN - uint8(max)
				s.ctl = ctlCleanupDiscard
				return
			}
		}
		s.finishCleanup(k)
	default:
		panic("kernel: bad step")
	}
}

// endStep empties mana pools and moves to step next.
//
//go:noinline
func (s *State) endStep(next kop.Step) {
	if s.fbR[0]|s.fbR[1]|s.pmOn != 0 {
		s.firebendEnd(next) // firebending red survives the combat steps, PersistentMana$ units every step (kw_firebend.go, mana_persistent.go)
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		s.pl[p].pool = [kop.NumManaKinds]uint8{}
		s.rbClear(p)
	}
	s.g.poolArt = [MaxSeats]uint8{}
	s.g.poolTre = [MaxSeats][kop.NumManaKinds]uint8{}
	s.step = next
	s.ctl = ctlStepBegin
}

// advance ends the current step after a full priority round.
func (s *State) advance(k *prep) {
	next := s.step + 1 // (first-strike damage -> damage, end of combat -> main 2, end -> cleanup)
	switch s.step {
	case kop.StepAttackers:
		if !s.declared {
			next = kop.StepEndCombat
		}
	case kop.StepBlockers:
		s.fsStep = s.anyFirstStrike(k)
		if !s.fsStep {
			next = kop.StepDamage
		}
	case kop.StepFirstStrikeDamage:
		s.endStep(next) // a step of its own, so the pools empty as it ends (CR 500.4, 510.4)
		return
	case kop.StepEndCombat:
		s.clearCombat()
	case kop.StepCleanup:
		// a cleanup priority round (triggers) ends: another cleanup step
		next = kop.StepCleanup
	}
	if k.gen != nil && k.gen.anyAddPhase {
		next = s.addPhaseNext(next) // extra phases and the phase ledger (act_addphase.go)
	} else if k.gen != nil && k.gen.anySkipPhase {
		next = s.skipCombatNext(next) // act_skipphase.go
	}
	s.endStep(next)
}

// pass is the priority holder passing.
func (s *State) pass(k *prep) {
	s.passes++
	if s.passes >= s.aliveCount() {
		if s.sp == 0 {
			s.advance(k)
			return
		}
		// a resolution is progress for gorge's hold-out rule (its events
		// clear suppressedCast), whether or not it writes an object
		// (Reckless Assault's 1 damage to a player): end the window
		s.heldTurn = ^uint16(0)
		s.resolveTop(k)
		return
	}
	s.prio = s.nextAlive(s.prio)
}

// afterResolve: priority returns to the active player (gorge legal_priority.go:73).
func (s *State) afterResolve() {
	s.prio = s.active
	s.passes = 0
	s.ctl = ctlPriority
}

func (s *State) finishCleanup(k *prep) {
	if k.gen != nil {
		s.cleanupGen(k)
	}
	if s.nkwx != 0 {
		// The until-end-of-turn keyword records are part of the permanent's
		// view (ObjHash and FeatGrantedRep read them), so mark each slot's
		// object before dropping them: a suppressed grant coming back after
		// a loss ended must not leave a per-object view cache stale.
		for i := 0; i < int(s.nkwx); i++ {
			sl := s.kwx[i].sl
			if sl != NoSlot && s.perms[sl].obj != NoObj {
				s.touchSl(sl)
			}
		}
		s.kwx, s.nkwx = [maxKwRepX]kwRepX{}, 0
	}
	// each slot's inline pump is zeroed below
	s.ptx, s.nptx = [maxPtX]ptRepX{}, 0

	s.mSlZero(k, mechRegen) // the shields' third bit (repl_regenerate.go), cleared with pfRegen below
	m := s.occ
	if k.phasing {
		m = m.or(s.phasedSlots()) // damage and until-end-of-turn effects end on a phased-out permanent too
	}
	for m.any() {
		sl := m.first()
		m = m.rest()
		p := &s.perms[sl]
		old := *p
		p.damage = 0
		if p.flags&pfDeathtouched != 0 {
			p.cseen |= cseenDT // gorge lists the removed mark at 0
		}
		p.flags &^= pfDeathtouched | pfDtMore | pfRegen | pfRegenX
		p.eotP, p.eotT, p.eotKw, p.eotKwRep = 0, 0, 0, [2]kop.KwMask{}
		if p.ovr.flags&ovSetPTEOT != 0 {
			p.ovr.flags &^= ovSetPTEOT
		}
		if p.ovr.flags&ovTypesEOT != 0 {
			p.ovr.flags &^= ovTypesEOT
			p.ovr.kw = 0
			if s.fxM.has(sl) && k.gen != nil {
				p.ovr.kw = s.copyExcKw(k, sl) // a copy's keyword exceptions stay (layer 1)
			}
		}
		p.ovr.flags &^= ovLoseAllEOT
		if *p != old {
			s.touchSl(sl)
		}
	}
	if s.g.nfx > 0 {
		s.ctrlCleanup(k) // until-end-of-turn and next-turn control records (CR 514.2; static_control.go)
	}
	if s.g.nfx > 0 {
		// the until-end-of-turn one-shot losses end with ovLoseAllEOT above
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.kind == fxLoseAll && r.r.eot() || r.kind == fxKwGrant })
	}
	if s.g.nfx > 0 {
		s.cloneCleanup(k) // until-end-of-turn copies end (CR 514.2; copy_clone.go)
	}
	if s.pmOn != 0 {
		// gorge keeps PersistentMana$ units through the cleanup step: a
		// 514.3a priority in it can still spend them (mana_persistent.go)
		s.persistEmpty(true)
	} else {
		for p := uint8(0); p < s.nSeats(); p++ {
			s.pl[p].pool = [kop.NumManaKinds]uint8{}
			s.rbClear(p)
		}
		s.fbR = [MaxSeats]uint8{} // already 0 (end of combat dropped it); explicit
		s.g.poolArt = [MaxSeats]uint8{}
		s.g.poolTre = [MaxSeats][kop.NumManaKinds]uint8{}
	}
	// CR 514.3a: the state-based check follows the 514.1/514.2 actions; when
	// one is performed the active player gets priority even with an empty
	// stack, and the step repeats (514.3b).
	t := s.touchN
	s.openPriority()
	if s.sba(k) || s.touchN != t {
		return
	}
	if s.npend > 0 {
		s.openPriority()
		return
	}
	next := s.nextAlive(s.active)
	if s.pl[s.active].extraTurns > 0 {
		s.pl[s.active].extraTurns--
		next = s.active
		if !DebugNoExtraTurnBegun {
			s.g.extraTurnBegun |= 1 << SeatIx(s.active) // ExYourExtraTurns: an extra turn's TurnChange fires
		}
	}
	s.beginTurn(next)
}

func (s *State) clearCombat() {
	m := s.occ
	for m.any() {
		sl := m.first()
		m = m.rest()
		p := &s.perms[sl]
		old := *p
		p.flags &^= pfAttacking | pfBlocking | pfBlocked | pfDealtFirst
		p.blocking = NoSlot
		p.nblock = 0
		if *p != old {
			s.touchSl(sl)
		}
	}
	s.xblkClear()
	s.nAttackers = 0
	s.fsStep = false
}

// firePhase enqueues "at the beginning of <step>" triggers.
func (s *State) firePhase(k *prep, st kop.Step) {
	if k.gen != nil {
		s.firePhaseGen(k, st)
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.subs[kop.TrigPhase].and(s.occ), &ord)
	for _, ls := range ord[:n] {
		lp := &s.perms[ls]
		if lp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
			continue
		}
		cd := &k.t.Cards[s.cardAt(ls)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigPhase || a.Trig.Step != st {
				continue
			}
			if !whoseOK(a.Trig.Whose, lp.ctrl, s.active) {
				continue
			}
			s.enqueue(lp.obj, s.objs[lp.obj].ver, s.cardAt(ls), uint8(i), lp.ctrl, NoObj, s.active, 0)
		}
	}
}

// sba performs state-based actions until none apply. It reports whether
// the game ended.
func (s *State) sba(k *prep) bool {
	if s.g.ndp != 0 && k.gen != nil && k.gen.anyDelaySpell {
		s.dropUncounterGone() // a protected spell that left the stack (repl_uncounterable.go)
	}
	for {
		if k.gen != nil && k.gen.ctrlSBA {
			s.ctrlSBA(k) // layer-2 control records and statics (static_control.go)
		}
		if s.cmdZoneAsk() { // CR 903.9a, before the lose checks (zone_cmdzone.go)
			return true
		}
		if s.g.lingerM.any() && !s.parkedWaiting() && !s.legendOwed(k) {
			// state-based checks unattach (each pass: a host that died in
			// the last one too), and a casualty's parked ask or a legend ask
			// is still that pass (CR 704.3, 704.5n, 704.5p): gorge's check
			// returns at its first order ask and its answer re-enters it
			s.lingerDrain(k)
		}
		// gorge's loss order (checkLoseConditions): every life loss, then
		// every empty-library one; a departing holder hands off as it is
		// marked (CR 726.4, act_initiative.go, before CR 725.4, act_monarch.go)
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.pl[p].life <= 0 && !s.pl[p].lost && !s.cantLoseLife(k, p) {
				s.pl[p].lost = true
				s.initLeft(k, p) // CR 726.4 (act_initiative.go)
				s.monarchLeft(k, p)
			}
		}
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.pl[p].drewEmpty {
				// drew from an empty library since the last check (CR 704.5b)
				s.pl[p].drewEmpty = false
				s.lose(p)
			}
		}
		if k.gen != nil && k.gen.anyInfect {
			s.poisonSBA(k) // CR 704.5c
		}
		s.cmdDamageSBA(k) // CR 704.6c (combat_cmddamage.go)
		s.checkOver()
		if s.over {
			if k.gen != nil || s.cfg&cfgCommander != 0 {
				s.ceaseTokensGen()
			}
			return true
		}
		if MaxSeats > 2 && s.sweepDeparted(k) {
			continue // CR 800.4a: the departed seats' objects left (zone_leavegame.go)
		}
		if (s.nAttackers > 0 || s.declared) && (k.gen == nil || k.gen.tyCombat) {
			s.removeNoncreatureCombatants(k) // CR 506.4 (cold)
		}
		var die, gy, pwz, aur, anni slotMask
		askAfter := false // exileOrderAsk parked casualties now: the ask follows this pass's batch
		// statics in force but none that can lower a toughness (kprio2)
		ptQuiet := s.statM.any() && s.sbaPTQuiet(k)
		m := s.occ
		for m.any() {
			sl := m.first()
			m = m.rest()
			p := &s.perms[sl]
			if p.p1 > 0 && p.ctrLow(int(kop.CtrM1M1)) > 0 {
				anni = anni.with(sl) // CR 704.5q, after the departures: a casualty leaves with its counters (CR 704.8)
			}
			t := s.typesAt(k, sl)
			if t&kop.TCreature != 0 {
				// toughness can only be <= damage or <= 0 when something
				// lowered it or damage is marked
				// A timestamped override record can set the base P/T (a
				// value-driven Animate: HandSize can be 0), so the printed
				// toughness is not enough for the no-damage shortcut.
				if p.damage == 0 && p.flags&pfDeathtouched == 0 && p.ctrLow(int(kop.CtrM1M1)) == 0 && p.eotT >= 0 && !s.fxM.has(sl) && (!s.statM.any() || ptQuiet && p.ovr.walk&walkOn == 0) &&
					p.ovr.flags&(ovSetPT|ovSetPTEOT|ovCopy) == 0 && k.cards[p.card].tough > 0 {
					goto noncreature
				}
				_, tough := s.pt(k, sl)
				if tough <= 0 {
					die = die.with(sl)
					gy = gy.with(sl)
					continue
				}
				// The deathtouch mark alone is lethal (CR 704.5h): a wither or
				// infect hit marks no damage (gorge destroyLethalDamage).
				if (int(p.damage) >= tough || p.flags&pfDeathtouched != 0) &&
					s.kw(k, sl)&kop.KwIndestructible == 0 {
					if s.destroyHeal(k, sl) {
						s.cfgx |= cfgxDestroyHealPass // CR 614.1: the counters leave, so a 0-toughness creature still dies next pass
						continue
					}
					if s.canRegen(sl) {
						s.regenerate(sl)
						continue
					}
					if k.gen != nil && k.gen.anyRepl&rbShield != 0 && s.shieldDestroy(sl) {
						s.cfgx |= cfgxShieldPass // CR 122.1c: the counter goes instead, the damage stays marked for the next pass
						continue
					}
					die = die.with(sl)
				}
			}
		noncreature:
			// one test for the common permanent: none of the three checks
			// below applies without an Aura, a planeswalker or an attachment
			if t&(kop.TAura|kop.TPlaneswalker) == 0 && p.attached == NoSlot {
				continue
			}
			if t&kop.TAura != 0 && s.atchSeat(k, sl) != 0 {
				// a player-attached Aura (the Enchant:Player curses): gorge
				// attachmentSBAs' HasAttachedPlayer arm (engine/rules/attach.go:128)
				// -- a lost seat cannot stay enchanted (CR 800.4a) and an
				// Aura whose own Enchant spec no longer admits the seat
				// (a control change moved a YouCtrl line) goes to the
				// graveyard like any illegal attachment. The player-keyword
				// protection grant gorge's playerProtectedFrom reads has no
				// kernel arm: no supported table grants one beside a curse.
				if !s.playerAuraLegal(k, sl) {
					gy = gy.with(sl)
					aur = aur.with(sl)
				}
			} else if t&kop.TAura != 0 && (p.attached == NoSlot || !s.enchantLegal(k, sl)) && !s.sbaBestowDetach(k, sl) {
				// unattached, or attached to something it can no longer
				// legally enchant (CR 704.5m: Imprisoned in the Moon makes
				// Witness Protection's creature a land); a bestowed Aura
				// detaches and stays instead (CR 702.103f)
				gy = gy.with(sl)
				aur = aur.with(sl)
			}
			if t&kop.TAura == 0 && p.attached != NoSlot && s.occ.has(p.attached) && s.typesAt(k, p.attached)&kop.TCreature == 0 && !s.legendOwed(k) {
				// an Equipment whose creature is no longer one (Imprisoned
				// in the Moon) becomes unattached and stays (CR 704.5n;
				// gorge attachmentSBAs; pair 9856). Under a legend ask it
				// stays attached: gorge parks the pass's batch with the ask
				// (CR 704.3), and the pass after the answer detaches it
				// (Crystallized Serah transformed beside another)
				h := p.attached
				s.touchSl(h)
				if k.gen != nil && k.gen.anyUnattach {
					// gorge's Equipment-bearer arm publishes the detach with
					// the bearer (CR 704.5n, attachmentSBAs)
					s.fireUnattached(k, sl, h)
				}
				p.attached = NoSlot
				s.touchSl(sl)
				if k.gen != nil && k.gen.anyAegis {
					s.aegisUnlink(k, sl, h, false) // zone_exilethenattach.go
				}
			}
			if t&kop.TPlaneswalker != 0 && p.ctrLow(int(kop.CtrLoyalty)) == 0 {
				gy = gy.with(sl)
				pwz = pwz.with(sl)
			}
		}
		if k.ctrMana {
			s.sbaCtrTough(k, kop.CtrM0M1, &die, &gy) // -0/-1 counters (mana_countercost.go)
		}
		if k.gen != nil && k.gen.anyM2M1 {
			s.sbaCtrTough(k, kop.CtrM2M1, &die, &gy) // -2/-1 counters (ctr_hone.go prepM2M1)
		}
		var wld slotMask
		if k.gen != nil && k.gen.anyWorld {
			wld = s.worldLosers(k) // CR 704.5k (zone_worldrule.go)
			gy = gy.or(wld)
		}
		if len(s.cold.parked) != 0 && (die.or(gy).any() || s.parkedWaiting()) {
			// the legend ask comes first (parkLegendChoice below); its answer
			// parks what competes
			if s.legendRule(k) {
				s.legDie = die
				return true
			}
			if s.exileOrderAsk(k, &die, &gy, &pwz, &aur, &askAfter) {
				return true
			}
		}
		wld = wld.and(gy) // a parked casualty left gy (exileOrderAsk)
		var bat slotMask
		if k.gen != nil && k.gen.anyBattle {
			// CR 704.5v as gorge models it (battleZeroDefense): exiled in a
			// batch of its own after the walkers (face_battle.go)
			bat = s.defeatedBattles(k)
			gy = gy.or(bat)
		}
		// CR 704.3, 704.5z: the Role losers leave in this pass's batch, after
		// the unattached Auras (gorge's roleSBA follows attachmentSBAs)
		var rolL [MaxPerm]Slot
		nrol := 0
		if k.gen != nil && s.g.roleOut != 0 {
			if rl, n := s.roleLosers(k); n > 0 {
				busy := die.or(pwz).or(bat).or(aur)
				for _, q := range rl[:n] {
					if !busy.has(q) {
						rolL[nrol] = q
						nrol++
						gy = gy.with(q)
					}
				}
			}
		}
		all := die.or(gy)
		if k.gen != nil && k.gen.equipAnim && s.sbaAttachedCreatures(k, all) && !all.any() {
			continue // CR 704.5p: an Equipment left its bearer; gorge's changed pass (static_unattachcreature.go)
		}
		if !all.any() {
			if s.cfgx&cfgxShieldPass != 0 {
				s.cfgx &^= cfgxShieldPass
				continue
			}
			if s.cfgx&cfgxDestroyHealPass != 0 {
				s.cfgx &^= cfgxDestroyHealPass
				continue
			}
			if k.gen != nil && k.gen.tyPost && s.tyPostRefresh(k) {
				continue // a detached bestow card is a creature again: the next pass sees it (gorge's changed pass)
			}
			// gorge poses the legend ask before the pass's other actions
			// apply, a token outside the battlefield still there (pair 3630)
			if s.legendRule(k) {
				s.legSaga = s.sagasDue(k) // parked with the ask (applyLegend)
				return true
			}
			// gorge's pass order (rules/sba.go): ceaseDeadTokens, then
			// checkSagas. A token Saga sacrificed below ceases on the next
			// pass either way; the order is only observable when both act.
			if k.gen != nil {
				s.sbaChainEnd(k)
			}
			if k.gen != nil || s.cfg&cfgCommander != 0 {
				s.ceaseTokensGen()
			}
			if k.anySaga && s.sagaSBA(k) {
				continue // CR 714.4 sacrificed a Saga: run the pass again
			}
			if initOn(k) {
				s.dungeonSBA(k) // CR 704.5t (act_initiative.go)
			}
			if k.anyDayNight && s.dnSBA(k) {
				continue // CR 702.145c/d/f/g (kw_daynight.go)
			}
			if anni.any() {
				s.annihilateAll(anni)
				continue
			}
			return false
		}
		// gorge poses the legend ask before any of the pass's casualties
		// apply (parkLegendChoice): park the creature casualties, which
		// the answer applies ahead of the non-kept legends
		if s.legendRule(k) {
			s.legDie = die
			s.legSaga = s.sagasDue(k)
			return true
		}
		if k.gen != nil || s.cfg&cfgCommander != 0 {
			s.ceaseTokensGen()
		}
		// gorge's order (rules/sba.go): the creature casualties (lethal
		// damage, zero toughness), then the world rule's losers, then
		// planeswalkers with no loyalty, then
		// defeated battles (to exile), then unattached Auras; within each, each seat's battlefield
		// list in seat order. The graveyard and the pending triggers follow it.
		var order [MaxPerm]Slot
		n := 0
		dw := die.or(wld)
		for _, gm := range [5]slotMask{die, wld.andNot(die), pwz.andNot(dw), bat.andNot(dw.or(pwz)), aur.andNot(dw.or(pwz).or(bat))} {
			for p := uint8(0); p < MaxSeats; p++ {
				n = s.battlefieldOrder(p, gm.and(s.ctrlM[p]), order[:], n)
			}
		}
		for _, q := range rolL[:nrol] {
			order[n] = q
			n++
		}
		var zs [MaxPerm]kop.Zone
		for i := 0; i < n; i++ {
			if bat.has(order[i]) {
				zs[i] = kop.ZoneExile // defeated, not destroyed: no dieZone
			} else {
				zs[i] = s.dieZone(k, order[i])
			}
		}
		// the creature casualties (the order's first nd) are one zone batch
		// (trig_casualtybatch.go); the rest stay outside it
		nd := 0
		var cb casBatch
		if k.gen != nil && (k.gen.anyBatch || k.gen.anyLinkRet) {
			if nd = die.count(); nd > 0 {
				cb = s.casOpen(k)
			}
		}
		for i := 0; i < n; i++ {
			if i == nd {
				cb.batchEnd(s)
			}
			s.fireLeave(k, order[i], zs[i])
		}
		cb.batchEnd(s)
		for i := 0; i < n; i++ {
			o := s.perms[order[i]].obj
			s.removePerm(k, order[i], zs[i], false)
			if bat.has(order[i]) {
				s.defeatQueue(k, o) // CR 310.11 (face_battle.go)
			}
			if i+1 == nd {
				cb.holdEnd(s, k)
			}
		}
		if anni.any() {
			s.annihilateAll(anni.and(s.occ))
		}
		if askAfter && s.parkedWaiting() {
			s.ctl = ctlExileOrder
			return true
		}
	}
}

// playerAuraLegal reports whether the player-attached Aura at sl is still
// legally attached (gorge attachmentSBAs' HasAttachedPlayer arm,
// engine/rules/attach.go:128, with playerAuraStillMatchesEnchant's read):
// the enchanted seat is alive and not lost, and the Aura's own Enchant
// restriction still admits it (a control change can move a YouCtrl line;
// gorge MatchesPlayerSpecFrom over the keyword's spec, whose corpus shapes
// are the Ctrl filters the kgen fold holds).
func (s *State) playerAuraLegal(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	q := s.atchSeat(k, sl) - 1
	if q >= s.nSeats() || s.pl[q].lost {
		return false
	}
	if f := k.t.Cards[s.cardAt(sl)].Enchant.Filter; f != 0 {
		switch k.t.Filters[f].Ctrl {
		case kop.CtrlYou:
			return q == p.ctrl
		case kop.CtrlOpp:
			return q != p.ctrl
		}
	}
	return true
}

// enchantLegal: Aura sl's host still matches its enchant ability (the
// filter for the Aura's controller; protection from everything excludes
// it). A lingering host (it left the battlefield, gorge unattaches at the
// next check) is left to that path.
func (s *State) enchantLegal(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	h := p.attached
	if !s.occ.has(h) || s.perms[h].obj == p.obj {
		return true
	}
	t := &k.t.Cards[s.cardAt(sl)].Enchant
	if k.gen != nil && k.gen.anyBestow && k.t.Cards[s.cardAt(sl)].HasBestow {
		t = &k.t.Cards[s.cardAt(sl)].BestowTgt // a bestowed Aura enchants a creature (kw_bestow.go)
	}
	if t.Kind != kop.TgtObject {
		return true
	}
	if k.gen != nil && k.gen.anyProtCol && s.protPrinted(k, h)&s.colorsGen(k, sl) != 0 {
		return false // printed protection from the Aura's colour (CR 702.16c; gorge attachmentSBAs)
	}
	if k.gen != nil && k.gen.anyProtF && s.protQualityAt(k, h, sl, s.cardAt(sl), true) {
		return false
	}
	return s.matchPerm(k, t.Filter, h, p.ctrl, NoSlot) && s.kw(k, h)&kop.KwProtectionAll == 0
}

// leaveAll moves the permanents of mask to zone z at once (leave triggers
// look back over all of them), in gorge's order: player 0's battlefield
// list, then player 1's.
func (s *State) leaveAll(k *prep, mask slotMask, z kop.Zone) {
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, mask.and(s.ctrlM[p]), order[:], n)
	}
	if len(s.cold.parked) != 0 && z == kop.ZoneGraveyard && s.res.active {
		first := n
		for i, sl := range order[:n] {
			if s.graveMoveCompetes(k, s.perms[sl].obj) {
				first = i
				break
			}
		}
		for _, sl := range order[first:n] {
			o := s.perms[sl].obj
			if s.park(k, o, parkMass, 0) {
				continue
			}
			if s.parkMassQuiet(o) == nil {
				s.capFull(StallReplCands)
				break
			}
		}
		// Gorge suspends the whole destroy-all at the first victim whose move
		// competes (CR 616.1): that victim and every later one wait for its
		// answer, competing or not. Only the earlier ones move now.
		n = first
	}
	// A commander heading to a hand or library under CR 903.9b stops the sweep
	// at its own move: the parked MoveZone stays on the battlefield, its leave
	// triggers fire after the answer, and the instruction runs again from there.
	hasCmd := false
	if s.cfg&cfgCommander != 0 && (z == kop.ZoneHand || z == kop.ZoneLibrary) {
		for _, sl := range order[:n] {
			if s.cmdZoneCommander(s.perms[sl].obj) {
				hasCmd = true
				break
			}
		}
	}
	parkAt := -1
	if hasCmd {
		for i := 0; i < n; i++ {
			o := s.perms[order[i]].obj
			if s.cmdZoneCommander(o) && !s.cmdZoneAnswered(o) {
				parkAt = i
				break
			}
		}
	}
	end := n
	if parkAt >= 0 {
		end = parkAt
	}
	hr := s.holdOpen(k)
	if !hasCmd {
		defer s.holdClose(k, hr)
		if k.gen != nil && k.gen.anyBatch {
			s.leaveAllBatched(k, order[:n], z)
			return
		}
	}
	var zs [MaxPerm]kop.Zone
	for i := 0; i < n; i++ {
		zs[i] = z
		if z == kop.ZoneGraveyard {
			zs[i] = s.dieZone(k, order[i])
		}
	}
	if hasCmd {
		// the answered commander(s) in the prefix take their destination now:
		// mode 0 rewrites it to the command zone
		for i := 0; i < end; i++ {
			if o := s.perms[order[i]].obj; s.cmdZoneAnswered(o) {
				bb := false
				s.cmdZoneReplace(k, o, &zs[i], &bb)
			}
		}
	}
	for i := 0; i < end; i++ {
		s.fireLeave(k, order[i], zs[i])
	}
	for i := 0; i < end; i++ {
		s.removePerm(k, order[i], zs[i], false)
	}
	if parkAt < 0 {
		if hasCmd {
			s.holdClose(k, hr)
		}
		return
	}
	// close the departure frame before the park: a held return would take the
	// one park slot, and a commander move beside one stalls by name
	s.holdClose(k, hr)
	if s.res.ask != raNone || s.g.pre.state != 0 {
		s.stall(StallLowering)
		return
	}
	zz, bb := z, false
	if !s.cmdZoneReplace(k, s.perms[order[parkAt]].obj, &zz, &bb) {
		s.stall(StallLowering) // the scanned slot was a parkable commander
	}
}

// battlefieldOrder appends the slots of mask (controller p's) to out from
// n, in p's battlefield list order (entry order), and returns the new
// length.
func (s *State) battlefieldOrder(p uint8, mask slotMask, out []Slot, n int) int {
	n0 := n
	for m := mask; m.any(); m = m.rest() {
		sl := m.first()
		j := n
		for j > n0 && s.perms[out[j-1]].ts > s.perms[sl].ts {
			out[j] = out[j-1]
			j--
		}
		out[j] = sl
		n++
	}
	return n
}

func (s *State) regenerate(sl Slot) {
	s.touchSl(sl)
	p := &s.perms[sl]
	if p.flags&pfDeathtouched != 0 {
		p.cseen |= cseenDT // gorge lists the removed mark at 0
	}
	s.spendRegenShield(sl)
	s.leaveCombat(sl) // CR 701.19b: regeneration removes it from combat
	p.flags &^= pfDeathtouched | pfDtMore
	wasUntapped := p.flags&pfTapped == 0
	fire := wasUntapped && s.g.on && s.subs[kop.TrigTaps].any()
	if fire {
		defer func() {
			s.fireTaps(s.k(), sl) // gorge regeneration taps it (EmitTap)
			s.markTapped(s.k(), sl)
		}()
	}
	p.flags |= pfTapped
	if wasUntapped && !fire {
		s.markTapped(s.k(), sl)
	}
	p.damage = 0
}

// fireDelayed puts "at the beginning of the next end step" delayed
// triggers on the stack like gorge (trigger_delayed.go checkDelayedTriggers
// -> DelayedPush): one triggered ability per registration, source = the
// creating object of an AtEOT$ rider (CR 603.7d-e), else the affected
// object, ability index abDelayed (255 in the Projection), controlled by the
// registering player; it performs its action on resolution if the affected
// object is still the same permanent.
func (s *State) fireDelayed(k *prep) {
	n := s.ndly
	s.ndly = 0
	var keep uint8
	var fol [MaxDelayed]delayed
	var nf uint8
	if k.gen != nil && (k.gen.anyEarthbend || k.gen.anyLeaveRet) {
		n, keep = s.splitLeaveDelayed(n) // leave-keyed records survive the end step
	}
	for i := 0; i < int(n); i++ {
		d := s.dly[i]
		if d.when == dlyGrpFollow || d.when == dlyUpkeep {
			fol[nf] = d // carried by its head's trigger (trig_tokenateot.go), or due at the next upkeep (trig_upkeepsac.go)
			nf++
			continue
		}
		if d.when == dlyEndCombat && k.gen != nil && k.gen.anyTokEndCombat {
			continue // gorge's |TT= expiry: this turn's end of combat has passed
		}
		if d.act == uint8(kop.OpNop) {
			// an empty-set registration: the source's trigger, doing nothing
			s.enqueue(d.obj, uint8(d.ver), s.objs[d.obj].card, abDelayed, d.ctrl, NoObj, d.ctrl, int16(d.act))
			continue
		}
		if k.gen != nil && objWord(s.objs[d.obj].ver) != d.ver && !staleOrdered(kop.Op(d.act)) && d.when != dlyUntracked {
			// gorge drops a generated delayed program whose object left (a
			// token copy that died). A keyword promise (Warp, Unearth) is not tied to
			// its source's battlefield incarnation (CR 603.7): it still takes
			// part in the end step's trigger order, is put on the stack and
			// resolves doing nothing for the stale incarnation (CR 603.7;
			// gorge's DelayedPush stamps the registered incarnation). A
			// token's AtEOT$ Destroy (__kwAtEOTDestroy) is the same.
			continue
		}
		m := s.npend
		s.enqueue(dlySource(&d), uint8(d.ver), s.objs[d.obj].card, abDelayed, d.ctrl, d.obj, d.ctrl, delayedAmt(d))
		s.echoOwnDelayed(k, &d, m)
	}
	if keep != 0 {
		copy(s.dly[:keep], s.dly[n:n+keep])
		s.ndly = keep
	}
	if nf != 0 {
		copy(s.dly[s.ndly:], fol[:nf])
		s.ndly += nf
	}
}

// resolveDelayed performs a delayed trigger's action.
func (s *State) resolveDelayed(k *prep, e *stackEntry) {
	if e.trigAmt == speedTrigAmt && e.obj == NoObj {
		s.resolveSpeed(e) // the source-less speed trigger (kw_speed.go)
		return
	}
	if monarchTrig(e) {
		s.resolveMonarchTrig(k, e) // the monarch's draw or steal (act_monarch.go)
		return
	}
	if k.gen != nil && e.trigAmt == opLeaveReturn {
		s.leaveReturn(k, e) // trig_leavereturn.go: the card is in a graveyard
		return
	}
	if k.gen != nil && kop.Op(e.trigAmt) == kop.OpEarthbendReturn {
		s.earthbendReturn(k, e) // before the battlefield check: the land is in a graveyard or exile
		return
	}
	if e.trigAmt == opSacUntracked {
		s.resolveSacUntracked(k, e) // trig_ateotmovers.go
		return
	}
	if kop.Op(e.trigAmt) == kop.OpNop {
		return // registered over an empty set: does nothing
	}
	if kop.Op(e.trigAmt) == kop.OpCopySpell {
		// casualty's trigger (CR 702.153a, kw_casualty.go): copy the spell
		// it was cast with, new targets allowed
		if k.gen != nil {
			s.copySpell(k, e.trigObj, 1, true) // false: the election is posed, and resolveTop reads the ask
		}
		return
	}
	ref := dlyRef(e.obj, e.trigObj) // the affected object, not a creator source
	ob := &s.objs[ref]
	if ob.ver != e.srcVer || ob.zone != kop.ZoneBattlefield {
		return
	}
	sl := ob.slot
	switch kop.Op(e.trigAmt) {
	case kop.OpDestroy:
		s.destroyDelayed(k, sl) // trig_tokenateot.go
	case kop.OpSacrifice:
		if s.sacBlocked(k, sl) {
			return // gorge effSacrifice skips a permanent that can't be sacrificed (static_cantsacrifice.go)
		}
		s.fireSac(k, sl)
		s.leave(k, sl, kop.ZoneGraveyard, false)
	case kop.OpExile, kop.OpUnearth:
		s.leave(k, sl, kop.ZoneExile, false) // OpUnearth: CR 702.84a's end-step exile
	case kop.OpWarpExile:
		// CR 702.185a: the warped permanent is exiled at the beginning of
		// the next end step; its owner may cast it from exile on a later
		// turn (kw_warp.go)
		s.leave(k, sl, kop.ZoneExile, false)
		if k.gen != nil && k.gen.anyWarp && s.objs[ref].zone == kop.ZoneExile {
			s.warpRecastGrant(ref)
		}
	case kop.OpBounce, opDashReturn:
		s.leave(k, sl, kop.ZoneHand, false) // opDashReturn: CR 702.109a's end-step return (kw_dash.go)
	default:
		panic("kernel: delayed trigger action not implemented: " + kop.Op(e.trigAmt).String())
	}
}

// legendRule finds a player controlling two or more legendary permanents
// with the same name (CR 704.5j); it poses the keep choice (ctlLegend) and
// reports true, or reports false when there is none.
func (s *State) legendRule(k *prep) bool { return s.legendGroup(k, false) }

// legendGroup is legendRule; peek poses nothing.
func (s *State) legendGroup(k *prep, peek bool) bool {
	if !s.legM.many() && (k.gen == nil || !k.gen.legendGrant) {
		return false // no printed legendary and no effect can grant the supertype
	}
	for p := uint8(0); p < s.nSeats(); p++ { // gorge legendGroups: AliveFrom(0)
		if s.out(p) {
			continue
		}
		// gorge asks the groups in the battlefield order of each group's
		// FIRST member (legendGroups appends a group when its name is
		// first seen), not of the first duplicate: two groups made at once
		// (Leonardo's Technique returning two legends) are asked that way.
		// perm.ts is that list's order (zones.go joined); slots are reused,
		// so slot order alone can differ from the entry order.
		var seen [MaxPerm]CardID
		var dup [MaxPerm]bool
		var ord [MaxPerm]Slot
		cand := s.ctrlM[p].and(s.legM)
		if k.gen != nil && k.gen.legendGrant {
			cand = s.ctrlM[p] // a derived Legendary is not in legM (printed types only)
		}
		no := s.battlefieldOrder(p, cand, ord[:], 0)
		n := 0
		for i := 0; i < no; i++ {
			sl := ord[i]
			if s.typesAt(k, sl)&kop.TLegendary == 0 {
				continue
			}
			c := s.nameCardAt(sl) // by name: Witness Protection renames (CR 704.5j; pair 28)
			dupe := false
			for j := 0; j < n; j++ {
				if seen[j] == c {
					dup[j] = true
					dupe = true
					break
				}
			}
			if dupe {
				continue
			}
			seen[n] = c
			n++
		}
		for i := 0; i < n; i++ {
			if dup[i] {
				if !peek {
					s.legP, s.legCard = p, seen[i]
					s.ctl = ctlLegend
				}
				return true
			}
		}
	}
	return false
}

// legendOwed reports whether this pass has a legend ask to pose.
func (s *State) legendOwed(k *prep) bool { return s.legendGroup(k, true) }

func (s *State) legendOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChLegend, s.legP
	// battlefield scan order, as gorge asks the keep options (g.ids)
	var ord [MaxPerm]Slot
	no := s.battlefieldOrder(s.legP, s.ctrlM[s.legP], ord[:], 0)
	for i := 0; i < no; i++ {
		sl := ord[i]
		if s.nameCardAt(sl) == s.legCard && s.typesAt(k, sl)&kop.TLegendary != 0 {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
}

// applyLegend keeps the chosen permanent; the others go to the graveyard.
func (s *State) applyLegend(k *prep, a Action) {
	// the pose pass's out-of-battlefield tokens and copies ride the legend
	// batch (gorge parkLegendChoice parks ceaseDeadTokens' finds with the
	// ask and applyLegendBatch applies them with the answer): they cease
	// here, before the next pass poses a further group, whose graveyard
	// order they would otherwise shift (CR 704.3, 704.5d). A token this
	// batch itself removes is still on the battlefield here and ceases on
	// the pass after the answer, as in gorge.
	if k.gen != nil || s.cfg&cfgCommander != 0 {
		s.ceaseTokensGen()
	}
	keep := Obj(a.Source())
	var out slotMask
	for m := s.ctrlM[s.legP]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].obj != keep && s.nameCardAt(sl) == s.legCard && s.typesAt(k, sl)&kop.TLegendary != 0 {
			out = out.with(sl)
		}
	}
	// one batch (gorge applyLegendBatch): the parked creature casualties
	// first, then the non-kept legends; a casualty that is a non-kept
	// duplicate is removed once, as a legend
	die := s.legDie.andNot(out)
	s.legDie = slotMask{}
	var zs [MaxPerm]kop.Zone
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, die.and(s.ctrlM[p]), order[:], n)
	}
	// the non-kept legends in battlefield (entry) order, as gorge walks g.ids
	n = s.battlefieldOrder(s.legP, out, order[:], n)
	if len(s.cold.parked) != 0 {
		kept := order[:0]
		for _, sl := range order[:n] {
			if !s.park(k, s.perms[sl].obj, parkLeave, 0) {
				kept = append(kept, sl)
			}
		}
		n = len(kept)
	}
	for i := 0; i < n; i++ {
		zs[i] = s.dieZone(k, order[i])
	}
	// the Sagas the same state-based check sacrificed join the batch (gorge
	// parkLegendChoice parks the pass's acts with the ask): the legend ruled
	// out sees them go (CR 704.3), they see it go
	sag := s.legSaga.and(s.occ).andNot(die).andNot(out)
	s.legSaga = slotMask{}
	nl := n
	for p := uint8(0); p < MaxSeats; p++ {
		n = s.battlefieldOrder(p, sag.and(s.ctrlM[p]), order[:], n)
	}
	for i := nl; i < n; i++ {
		zs[i] = s.dieZone(k, order[i])
	}
	for i := 0; i < n; i++ {
		if i >= nl {
			s.fireSac(k, order[i])
		}
		s.fireLeave(k, order[i], zs[i])
	}
	for i := 0; i < n; i++ {
		s.removePerm(k, order[i], zs[i], false)
	}
	s.ctl = ctlPriority
	if s.parkedWaiting() {
		s.ctl = ctlExileOrder
	}
}
