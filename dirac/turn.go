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
	if s.k().anyYourTurns && s.g.yourTurns[p&1] < 255 {
		s.g.yourTurns[p&1]++ // ExYourTurns: this turn is begun with p active
	}
	if s.k().anyRedNC {
		s.g.redNC = [2]uint8{} // ExRedNCDmg: a fresh turn's tally
	}
	if s.k().anyDayNight {
		s.dnTurn() // CR 731.2's count, before the bank resets (kw_daynight.go)
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
	s.step = kop.StepUntap
	s.ctl = ctlStepBegin
	s.carry = carryNone
	for m := s.occ; m.any(); m = m.rest() {
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
		pl.landsPlayed = 0
		pl.drawnTurn = 0
		pl.lifeGained = 0
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
		if k.phasing {
			s.phaseUntap(k) // CR 702.26a/p, before the untap action (kw_phasing.go)
		}
		if k.anyDayNight && s.g.dayNight&3 != 0 {
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
				if s.g.nlA[0] != 0 {
					s.netDrop(p.obj) // the lock ends at the first untap (act_lockpermanent.go)
				}
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
		s.firePhase(k, kop.StepUpkeep)
		if k.gen != nil && k.gen.anyDelayUp {
			s.fireDelayedUpkeep(k)
		}
		if initOn(k) {
			s.initUpkeep(k) // CR 726.2 (act_initiative.go)
		}
		s.openPriority()
	case kop.StepDraw:
		if s.turn == 1 {
			s.step = kop.StepMain1
			return
		}
		s.draw(s.active)
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
		if s.anyBlock(k) {
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
	s.pl[0].pool, s.pl[0].poolR, s.poolRx[0] = [kop.NumManaKinds]uint8{}, 0, 0
	s.pl[1].pool, s.pl[1].poolR, s.poolRx[1] = [kop.NumManaKinds]uint8{}, 0, 0
	s.g.poolArt = [2]uint8{}
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
	}
	s.endStep(next)
}

// pass is the priority holder passing.
func (s *State) pass(k *prep) {
	s.passes++
	if s.passes >= 2 {
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
	s.prio = 1 - s.prio
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
		s.pl[0].pool, s.pl[0].poolR, s.poolRx[0] = [kop.NumManaKinds]uint8{}, 0, 0
		s.pl[1].pool, s.pl[1].poolR, s.poolRx[1] = [kop.NumManaKinds]uint8{}, 0, 0
		s.fbR = [2]uint8{} // already 0 (end of combat dropped it); explicit
		s.g.poolArt = [2]uint8{}
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
	next := 1 - s.active
	if s.pl[s.active].extraTurns > 0 {
		s.pl[s.active].extraTurns--
		next = s.active
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
	for {
		if k.gen != nil && k.gen.ctrlSBA {
			s.ctrlSBA(k) // layer-2 control records and statics (static_control.go)
		}
		if s.g.lingerM.any() && !s.parkedWaiting() && !s.legendOwed(k) {
			// state-based checks unattach (each pass: a host that died in
			// the last one too), and a casualty's parked ask or a legend ask
			// is still that pass (CR 704.3, 704.5n, 704.5p)
			if k.phasing {
				s.lingerKeepPhased() // a phased-out attachment is not checked (kw_phasing.go)
			} else {
				for m := s.g.lingerM; m.any(); m = m.rest() {
					s.touchSl(m.first())
				}
				s.g.lingerM = slotMask{}
				s.g.nlinger = 0
			}
		}
		for p := uint8(0); p < 2; p++ {
			if s.pl[p].life <= 0 && !s.pl[p].lost && !s.cantLose(k, p) {
				s.pl[p].lost = true
			}
			if s.pl[p].drewEmpty {
				// drew from an empty library since the last check (CR 704.5b)
				s.pl[p].drewEmpty = false
				s.lose(p)
			}
		}
		if k.gen != nil && k.gen.anyInfect {
			s.poisonSBA(k) // CR 704.5c
		}
		s.checkOver()
		if s.over {
			if k.gen != nil {
				s.ceaseTokensGen()
			}
			return true
		}
		if (s.nAttackers > 0 || s.declared) && (k.gen == nil || k.gen.tyCombat) {
			s.removeNoncreatureCombatants(k) // CR 506.4 (cold)
		}
		var die, gy, pwz, aur, anni slotMask
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
				if p.damage == 0 && p.ctrLow(int(kop.CtrM1M1)) == 0 && p.eotT >= 0 && !s.fxM.has(sl) && (!s.statM.any() || ptQuiet && p.ovr.walk&walkOn == 0) &&
					p.ovr.flags&(ovSetPT|ovSetPTEOT|ovCopy) == 0 && k.cards[p.card].tough > 0 {
					goto noncreature
				}
				_, tough := s.pt(k, sl)
				if tough <= 0 {
					die = die.with(sl)
					gy = gy.with(sl)
					continue
				}
				if (int(p.damage) >= tough || (p.flags&pfDeathtouched != 0 && p.damage > 0)) &&
					s.kw(k, sl)&kop.KwIndestructible == 0 {
					if p.flags&pfRegen != 0 {
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
			if t&kop.TAura != 0 && (p.attached == NoSlot || !s.enchantLegal(k, sl)) && !s.sbaBestowDetach(k, sl) {
				// unattached, or attached to something it can no longer
				// legally enchant (CR 704.5m: Imprisoned in the Moon makes
				// Witness Protection's creature a land); a bestowed Aura
				// detaches and stays instead (CR 702.103f)
				gy = gy.with(sl)
				aur = aur.with(sl)
			}
			if t&kop.TAura == 0 && p.attached != NoSlot && s.occ.has(p.attached) && s.typesAt(k, p.attached)&kop.TCreature == 0 {
				// an Equipment whose creature is no longer one (Imprisoned
				// in the Moon) becomes unattached and stays (CR 704.5n;
				// gorge attachmentSBAs; pair 9856)
				h := p.attached
				s.touchSl(h)
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
			s.sbaCtrTough(k, &die, &gy) // -0/-1 counters (mana_countercost.go)
		}
		if len(s.cold.parked) != 0 && (die.or(gy).any() || s.parkedWaiting()) {
			// the legend ask comes first (parkLegendChoice below); its answer
			// parks what competes
			if s.legendRule(k) {
				s.legDie = die
				return true
			}
			if s.exileOrderAsk(k, &die, &gy, &pwz, &aur) {
				return true
			}
		}
		var bat slotMask
		if k.gen != nil && k.gen.anyBattle {
			// CR 704.5v as gorge models it (battleZeroDefense): exiled in a
			// batch of its own after the walkers (face_battle.go)
			bat = s.defeatedBattles(k)
			gy = gy.or(bat)
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
			if k.gen != nil && k.gen.tyPost && s.tyPostRefresh(k) {
				continue // a detached bestow card is a creature again: the next pass sees it (gorge's changed pass)
			}
			// gorge poses the legend ask before the pass's other actions
			// apply, a token outside the battlefield still there (pair 3630)
			if s.legendRule(k) {
				return true
			}
			// roleSBA runs only after the legend answer (rules/sba.go)
			if k.gen != nil && s.g.roleOut != 0 && s.roleSBA(k) {
				continue // CR 704.5z: a Role left; run the pass again (static_roletokens.go)
			}
			// gorge's pass order (rules/sba.go): ceaseDeadTokens, then
			// checkSagas. A token Saga sacrificed below ceases on the next
			// pass either way; the order is only observable when both act.
			if k.gen != nil {
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
			return true
		}
		if k.gen != nil {
			s.ceaseTokensGen()
		}
		// gorge's order (rules/sba.go): the creature casualties (lethal
		// damage, zero toughness), then planeswalkers with no loyalty, then
		// defeated battles (to exile), then unattached Auras; within each, player 0's battlefield list then
		// player 1's. The graveyard and the pending triggers follow it.
		var order [MaxPerm]Slot
		n := 0
		for _, gm := range [4]slotMask{die, pwz.andNot(die), bat.andNot(die.or(pwz)), aur.andNot(die.or(pwz).or(bat))} {
			for p := uint8(0); p < 2; p++ {
				n = s.battlefieldOrder(p, gm.and(s.ctrlM[p]), order[:], n)
			}
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
	}
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
	if k.gen != nil && k.gen.anyProtF && s.protQuality(k, h, sl, s.cardAt(sl)) {
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
	for p := uint8(0); p < 2; p++ {
		n = s.battlefieldOrder(p, mask.and(s.ctrlM[p]), order[:], n)
	}
	if len(s.cold.parked) != 0 && z == kop.ZoneGraveyard && s.res.active {
		m := 0
		for _, sl := range order[:n] {
			if !s.park(k, s.perms[sl].obj, parkMass, 0) {
				order[m] = sl
				m++
			}
		}
		n = m
	}
	hr := s.holdOpen(k)
	defer s.holdClose(k, hr)
	if k.gen != nil && k.gen.anyBatch {
		s.leaveAllBatched(k, order[:n], z)
		return
	}
	var zs [MaxPerm]kop.Zone
	for i := 0; i < n; i++ {
		zs[i] = z
		if z == kop.ZoneGraveyard {
			zs[i] = s.dieZone(k, order[i])
		}
	}
	for i := 0; i < n; i++ {
		s.fireLeave(k, order[i], zs[i])
	}
	for i := 0; i < n; i++ {
		s.removePerm(k, order[i], zs[i], false)
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
	if p.flags&pfTapped == 0 && s.g.on && s.subs[kop.TrigTaps].any() {
		defer s.fireTaps(s.k(), sl) // gorge regeneration taps it (EmitTap)
	}
	p.flags |= pfTapped
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
	if k.gen != nil && (k.gen.anyEarthbend || k.gen.anyLeaveRet) {
		n, keep = s.splitLeaveDelayed(n) // leave-keyed records survive the end step
	}
	for i := 0; i < int(n); i++ {
		d := s.dly[i]
		if d.when == dlyEndCombat && k.gen != nil && k.gen.anyTokEndCombat {
			continue // gorge's |TT= expiry: this turn's end of combat has passed
		}
		if d.act == uint8(kop.OpNop) {
			// an empty-set registration: the source's trigger, doing nothing
			s.enqueue(d.obj, d.ver, s.objs[d.obj].card, abDelayed, d.ctrl, NoObj, d.ctrl, int16(d.act))
			continue
		}
		if k.gen != nil && s.objs[d.obj].ver != d.ver && !staleOrdered(kop.Op(d.act)) && d.when != dlyUntracked {
			// gorge drops a generated delayed program whose object left (a
			// token copy that died). A keyword promise (Warp, Unearth) is not tied to
			// its source's battlefield incarnation (CR 603.7): it still takes
			// part in the end step's trigger order, and takePending then
			// mints no trigger for the stale incarnation (gorge's
			// foldDelayedPush checks TrackSource at push time). A token's
			// AtEOT$ Destroy (__kwAtEOTDestroy) is the same:.
			continue
		}
		s.enqueue(dlySource(&d), d.ver, s.objs[d.obj].card, abDelayed, d.ctrl, d.obj, d.ctrl, delayedAmt(d))
	}
	if keep != 0 {
		copy(s.dly[:keep], s.dly[n:n+keep])
		s.ndly = keep
	}
}

// resolveDelayed performs a delayed trigger's action.
func (s *State) resolveDelayed(k *prep, e *stackEntry) {
	if e.trigAmt == speedTrigAmt && e.obj == NoObj {
		s.resolveSpeed(e) // the source-less speed trigger (kw_speed.go)
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
	case kop.OpBounce:
		s.leave(k, sl, kop.ZoneHand, false)
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
	for _, p := range [2]uint8{0, 1} { // gorge legendGroups: AliveFrom(0)
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
			d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
		}
	}
}

// applyLegend keeps the chosen permanent; the others go to the graveyard.
func (s *State) applyLegend(k *prep, a Action) {
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
	for p := uint8(0); p < 2; p++ {
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
	for i := 0; i < n; i++ {
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
