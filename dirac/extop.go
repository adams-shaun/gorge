package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The extension op (kop.OpExt): A.N names the sub-op
// (kop/ext.go). Cold and out of line, the last exec chain link
// (execStdmech's OpExt case): a table with no ext instruction never
// reaches it.

// execExt runs one OpExt instruction. One case per sub-op, sorted by
// ordinal: a new sub-op adds its case at its own ordinal position.
//
//go:noinline
func (s *State) execExt(k *prep, in *kop.Instr) (ok, done bool) {
	switch kop.ExtOp(in.A.N) {
	case kop.ExtPreventColor:
		// gorge effChooseColor: the controller chooses (always five
		// colours here), then the Effect registers the prevention
		// (cast_altfree.go applyAskStdmech)
		s.res.ask, s.res.askP = raPrevColor, s.res.e.ctrl
		return false, true
	case kop.ExtPreventDamageScope:
		s.prevScopeAdd(k, in) // repl_preventscope.go
		return true, true
	case kop.ExtCounterPlusEOT:
		s.ctrPlusEOT(k) // ctr_pluseot.go
		return true, true
	case kop.ExtGrantPanharm:
		s.grantPanharm(k, in) // act_grantpanharm.go
		return true, true
	case kop.ExtGrantExile:
		s.grantExile(k, in) // cast_exilegrant.go
		return true, true
	case kop.ExtEachDamage:
		s.eachDamage(k, in)
		return true, true
	case kop.ExtRemoveCounterAny:
		return s.rcaStart(k, in), true // ctr_removeany.go
	case kop.ExtPutCounterExisting:
		return s.pceStart(k, in), true // ctr_putexisting.go
	case kop.ExtTwoPilesSac:
		return s.pilesStart(k, in), true // act_piles.go
	case kop.ExtChooseColorRes:
		s.res.ask, s.res.askP = raChooseColorRes, s.res.e.ctrl // static_colourgrant.go
		return false, true
	case kop.ExtColorGrant:
		s.colGrant(k, in) // static_colourgrant.go
		return true, true
	case kop.ExtExHaste:
		s.exHaste(k, in) // static_cantblockexcept.go
		return true, true
	case kop.ExtMayPlayGrave:
		s.mayPlayGrave(k, in) // cast_graveyardgrant.go
		return true, true
	case kop.ExtHideaway:
		if in.C == 1 {
			return s.hidePlay(k, in), true // kw_hideaway.go
		}
		return s.hideStart(k, in), true
	case kop.ExtTokenPT:
		s.tokenPT(k, in) // act_tokenpt.go
		return true, true
	case kop.ExtCopyTokenCtr:
		s.copyTokenCtr(k, in) // copy_counters.go
		return true, true
	case kop.ExtEndure:
		return s.endureStart(k, in), true // act_endure.go
	case kop.ExtDividedCounters:
		s.divCounters(k, in) // ctr_divided.go
		return true, true
	case kop.ExtStackExileMark:
		s.stackExileMark(k, in) // repl_stackexile.go
		return true, true
	case kop.ExtDmgMark:
		s.dmgMarkAdd(k, in) // repl_dmgmark.go
		return true, true
	case kop.ExtPlotGrant:
		s.plotGrant(k) // kw_plot.go
		return true, true
	case kop.ExtSacDelayed:
		s.sacDelayed(k) // kw_mobilize.go
		return true, true
	case kop.ExtCounterLose:
		s.counterLose(k, in) // act_counterlose.go
		return true, true
	case kop.ExtNumTimes:
		return s.numTimesStart(k, in), true // cast_announcetimes.go
	case kop.ExtCumulative:
		s.cumulativeStart(k, in) // trig_cumulative.go
		return true, true
	case kop.ExtCloneCopy:
		if in.B.N&kop.ClonePick != 0 {
			return s.clonePick(k, in), true // copy_clone.go
		}
		s.cloneCopy(k, in)
		return true, true
	case kop.ExtExileExMark:
		s.exileExMark(k, in) // repl_exilemark.go
		return true, true
	case kop.ExtAegisClone:
		return s.aegisStart(k, in), true // zone_exilethenattach.go
	case kop.ExtRecruit:
		return s.recruitStart(k), true // act_recruit.go
	case kop.ExtAirbend:
		s.airbend(k, in) // act_airbend.go
		return true, true
	case kop.ExtSweepFrom:
		s.sweepFrom(k, in) // act_sweepdamage.go
		return true, true
	case kop.ExtManaChoice:
		return s.manaChoiceStart(k, in), true // mana_colourchoice.go
	case kop.ExtLibSweep:
		s.libSweep(k, in) // zone_sweeptolibrary.go
		return true, true
	case kop.ExtBreachChoose:
		return s.breachStart(k), true // act_millthenreturn.go
	case kop.ExtBreachReturn:
		s.breachReturn(k)
		return true, true
	case kop.ExtExileLandChoose:
		return s.exileLandStart(k), true // act_exilelandchoose.go
	case kop.ExtPhases:
		s.phasesExt(k, in) // kw_phasing.go
		return true, true
	case kop.ExtLockActs:
		s.lockActs(k, in) // act_lockpermanent.go
		return true, true
	case kop.ExtMustBeBlocked:
		s.mustBlockExt(k, in) // combat_mustbeblocked.go
		return true, true
	case kop.ExtSpellToLib:
		s.spellToLib(k) // act_spelltolib.go
		return true, true
	case kop.ExtVote:
		return s.voteStart(k, in), true // act_vote.go
	case kop.ExtAttacksIfAble:
		s.attacksIfAbleExt(k, in) // combat_attacksifable.go
		return true, true
	case kop.ExtBlocksIfAble:
		s.blocksIfAbleExt(k, in) // combat_blocksifable.go
		return true, true
	case kop.ExtClearImprinted:
		s.g.nimp = 0
		return true, true
	case kop.ExtDigImprint:
		return true, true // read by the dig it follows (zone_digimprint.go)
	case kop.ExtRememberSearcher:
		s.rememberSearcher() // zone_remembersearcher.go
		return true, true
	case kop.ExtRememberAll:
		s.rememberAll(k, in) // act_rememberall.go
		return true, true
	case kop.ExtSuspect:
		s.suspectExt(k, in) // act_suspect.go
		return true, true
	case kop.ExtCopyAllEach:
		// the sweep's park halts the program at the OpExt (a copy's
		// as-enters ask, copy_allpark.go); the answer arm resumes it
		return !s.copyAllEach(k, in), true // copy_definedvalid.go
	case kop.ExtCopyExiledWith:
		// the exiled-with set is read once, seat 0 first; a parks only when
		// no later copy remains, so the ordinary copy ask resumes it
		return !s.copyExiledWith(k, in), true // copy_exiledwith.go
	case kop.ExtCopyAbility:
		return s.copyAbility(k, in), true // copy_ability.go
	case kop.ExtEffEtbCtr:
		s.effEtbCtr(k, in) // ctr_effectentry.go
		return true, true
	case kop.ExtFinalityCtr:
		s.finalityCtr(k, in) // trig_mayplaystatic.go
		return true, true
	case kop.ExtAddPhase:
		s.addPhaseGrant(k, in) // act_addphase.go
		return true, true
	case kop.ExtToGraveyard:
		s.moveRef(k, in.Obj, kop.ZoneGraveyard, false) // zone_toplibrary.go
		return true, true
	case kop.ExtChangeTargets:
		return s.changeTargets(k, in), true // act_stackrewrite.go
	case kop.ExtCounterAll:
		s.counterAll(k) // act_stackrewrite.go
		return true, true
	case kop.ExtExchangeControl:
		s.exchangeControl(k, in) // act_exchangecontrol.go
		return true, true
	case kop.ExtEnterUntapped:
		// the entering permanent untaps: not OpUntap, whose
		// stun substitution is wrong for an entrant (gorge effects/untap.go)
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			if p := &s.perms[s.objs[o].slot]; p.flags&pfTapped != 0 {
				p.flags &^= pfTapped
				s.touch(o)
			}
		}
		return true, true
	case kop.ExtSearchCounters:
		s.searchCounters(k, in) // ctr_search.go
		return true, true
	case kop.ExtEnterAttacking:
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.enterAttackingRepl(k, s.objs[o].slot, s.perms[s.objs[o].slot].ctrl) // repl_enterattacking.go
		}
		return true, true
	case kop.ExtUnlockDoor:
		return s.extUnlockDoor(k, in), true // face_unlockdoor.go
	case kop.ExtChooseNumber:
		// inside an as-enters program: asked before the next priority
		// (the OpChooseColor arm of execGen)
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			s.queueAsk(s.objs[o].slot, akNumber)
		}
		return true, true
	case kop.ExtChooseName:
		// the same, for the as-enters nonland card name (repl_choosename.go)
		if o := s.g.replaced; o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
			if kind := nameAskKind(in); kind != akNameHand || !s.nameHandNone(k, s.perms[s.objs[o].slot].ctrl) {
				s.queueAsk(s.objs[o].slot, kind)
			}
		}
		return true, true
	case kop.ExtTurnFaceUp:
		return s.extTurnFaceUp(k, in), true // face_turnupeffect.go
	case kop.ExtDistCounters:
		return s.distPickStart(k, in), true // ctr_distpick.go
	case kop.ExtExileEmbRem:
		s.exileEmbRem(k) // trig_emblemrem.go
		return true, true
	case kop.ExtCopyNamed:
		return !s.copyNamed(k, in), true // copy_named.go
	case kop.ExtCopyPicked:
		return !s.copyPicked(k, in), true // copy_picked.go
	case kop.ExtTurnRepl:
		s.turnRepl(k, in) // repl_turnrepl.go
		return true, true
	}
	panic("kernel: ext op not implemented: " + kop.ExtOp(in.A.N).String())
}

// eachDamage is ExtEachDamage: the damagers -- every battlefield
// permanent matching F in gorge's sweep order (sweepHits: seat 0 first, entry
// order), or, with damager slots in C's high byte, the legal
// objects of those target slots in pick order that are still on the
// battlefield (and match F when it is set) -- each deal B damage to the
// object in Ref C's low byte, one damage batch, each hit its own source
// (gorge effEachDamage: the damagers read once, the recipient skipped once
// it left the battlefield; dealHits' trigger and lifelink pass).
func (s *State) eachDamage(k *prep, in *kop.Instr) {
	dst := s.objRef(k, kop.Ref(in.C&0xff))
	if dst == NoSlot {
		return
	}
	pow := in.B.K == kop.VConst && in.B.N == 0 // each damager's own power (Count$CardPower)
	n := s.value(k, in.B)
	var ord [MaxPerm]Slot
	cnt := 0
	if mask := uint8(in.C >> 8); mask != 0 {
		cnt = s.eachDamageSlots(k, mask, in.F, &ord)
	} else {
		cnt = s.sweepHits(k, in.F, &ord)
	}
	var hs [MaxPerm]dmgHit
	m := 0
	for i := 0; i < cnt; i++ {
		a := n
		if pow {
			a = s.power(k, ord[i]) // read before any hit lands (one batch)
		}
		if a > 0 {
			hs[m] = dmgHit{src: ord[i], dst: dst, n: int16(min(a, 1<<14))}
			m++
		}
	}
	s.dealHitsToOne(k, hs[:m])
}

// eachDamageSlots lists the battlefield permanents the resolving entry's
// target slots in mask name, slot by slot in pick order (tgt[slot], then
// its extra objects), skipping targets illegal as the resolution started
// (res.bad, res.xbad: gorge binds the parent link's targets then, so
// Coordinated Clobbering's tap does not unmake its "untapped" damagers),
// objects that left since, duplicates and, when f is set, those not
// matching it (gorge eachDamagerTargets over parentLinkTargets /
// refTargetUnion).
func (s *State) eachDamageSlots(k *prep, mask uint8, f kop.FilterID, out *[MaxPerm]Slot) int {
	e := &s.res.e
	x := s.xtOf(e)
	src := s.objRef(k, kop.RefSelf)
	n := 0
	add := func(c, ver uint8) {
		if c == NoTarget || !isObjTarget(c) {
			return
		}
		ob := &s.objs[c]
		if ob.ver != ver || ob.zone != kop.ZoneBattlefield || !s.occ.has(ob.slot) {
			return
		}
		if f != 0 && !s.matchPerm(k, f, ob.slot, e.ctrl, src) {
			return
		}
		for j := 0; j < n; j++ {
			if out[j] == ob.slot {
				return
			}
		}
		if n < MaxPerm {
			out[n] = ob.slot
			n++
		}
	}
	for i := uint8(0); i < kop.MaxTargets; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		add(s.tgtCode(int(i)), e.tver[i])
		for j := 0; j < int(x.n); j++ {
			if x.slot[j] == i && s.res.xbad&(1<<j) == 0 {
				add(x.code[j], x.ver[j])
			}
		}
	}
	return n
}

// xtIllegal is the bit set of e's multi-object extras illegal now.
//
//go:noinline
func (s *State) xtIllegal(k *prep, e *stackEntry) (m uint8) {
	x := s.xtOf(e)
	for j := 0; j < int(x.n); j++ {
		if !s.xtLegal(k, e, j) {
			m |= 1 << j
		}
	}
	return m
}

// prepEachDamage sets anyEachTgt: some ExtEachDamage names damager target
// slots (C's high byte).
func prepEachDamage(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtEachDamage && in.C>>8 != 0 {
					g.anyEachTgt = true
					return
				}
			}
		}
	}
}
