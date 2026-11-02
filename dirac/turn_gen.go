package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Entering and leaving, turn bookkeeping, the as-enters choices and the
// Leyline ask of a generated table.

// ctlGenAsk is the control state of a generated-table decision posed
// between priority decisions (an as-enters choice, a Leyline).
const ctlGenAsk uint8 = 200

// ask kinds of genState.ask
const (
	akColor   uint8 = 1
	akType    uint8 = 2
	akEnchant uint8 = 3 // a returning Aura's host (s.g.pre only: CR 303.4f)
	akClone   uint8 = 4 // an "enter as a copy" choice, cast or returned (s.g.pre only; copy_asenters.go)
	akPayLife uint8 = 5 // a shock land's as-enters "pay N life or enter tapped" (repl_shockland.go)
	akMode    uint8 = 6 // a Siege's as-enters mode (face_siegemode.go)
	akReveal  uint8 = 7 // a reveal land's as-enters "reveal or enter tapped" (repl_revealland.go)
	akRevPick uint8 = 8 // the card a reveal land's paid election reveals
)

// enterGen runs after the core's enter placed o at sl, before its ETB triggers:
// it resets the slot's generated state, records how the permanent arrived,
// and applies the as-enters replacements (its own and other permanents').
func (s *State) enterGen(k *prep, sl Slot, o Obj, from kop.Zone, face uint8) {
	s.touch(o)
	s.clearFx(sl)
	s.arenaClrSlot(sl) // the slot takes a new permanent: no mechanic's mark for the last one survives (arena.go)
	if s.g.entExc != 0 {
		x := s.g.entExc
		s.g.entExc = 0
		s.copyExcAdd(k, sl, Slot(x>>8), uint8(x))
	}
	if k.gen.anyAegis {
		s.aegisLeaves(k, sl, o) // zone_exilethenattach.go
	}
	if s.g.nexw != 0 {
		s.exwEnter(o, from) // a source entering other than from the stack clears its forward links (zone_exiledwith.go)
	}
	if s.g.nchPick != 0 {
		s.charmPickLeft(sl) // modal_charmpick.go: a new permanent starts with no pick log (CR 400.7)
	}
	p := &s.perms[sl]
	p.chosen = 255
	s.g.cast.clr(sl)
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		s.g.cast.set(sl)
		if s.res.e.flags&sfKicked != 0 && !(k.gen.anyOptCost && castCollectedNever(k, &s.res.e)) {
			s.mSlSet(k, mechKicked, sl)
		}
		if sneakMarks(&s.res.e) {
			s.mSlSet(k, mechSneaked, sl) // kw_sneak.go
		}
	}
	s.g.zoneTurn.set(o)
	if s.g.enteredOn {
		s.noteEnter(o, kop.ZoneBattlefield, from, false)
	}
	if s.typesGen(k, sl)&kop.TLand == 0 {
		s.stashClr(k, o) // only a land played from exile keeps its stash counter (gorge)
	}
	if k.gen.anyEntryMerge && face == 0 {
		s.entryWinOpen(k, sl, p) // ctr_entryperkind.go: the printed and requested counters start the entry's one put per kind
	} else if n := s.g.enterCtr.n; n > 0 {
		if face == 0 {
			s.placeCounters(k, sl, int(s.g.enterCtr.kind), int(n), p.ctrl, ctrPutBy)
		}
		s.g.enterCtr.n = 0
	}
	if k.gen.anyEffEtbCtr && face == 0 && s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		s.effEtbEnter(k, sl, p) // ctr_effectentry.go: after the printed counters, before the entry's triggers
	}
	if k.gen.anyFinality && face == 0 && s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		s.finalityEnter(k, sl, p) // trig_mayplaystatic.go: the FINALITY rider, beside the P1P1 one
	}
	cid := s.cardAt(sl)
	g := k.gen
	if !g.has(cid, gfEnterRepl) && !s.statM.any() {
		if g.anyEntryMerge {
			s.entryWinClose(k, sl, p)
		}
		return
	}
	// as-enters replacements are applied before the object is on the
	// battlefield: counts made by them do not see it
	s.occ.clr(sl)
	s.ctrlM[p.ctrl].clr(sl)
	saveRep := s.g.replaced
	s.g.replaced = o
	var ovr uint8
	if g.anyEntUntap && face == 0 {
		ovr = s.entryOrderEnter(k, sl, o) // the CR 616.1 order ask's result, or a stall (repl_entryorder.go)
	}
	if face == 0 && g.has(cid, gfEnterRepl) {
		cd := &k.t.Cards[cid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplEntersProgram {
				ctx := evalCtx{src: o, srcSl: sl, ctrl: p.ctrl}
				if s.res.active {
					ctx.e = &s.res.e
				}
				ok := a.Repl.Cond == 0 || s.condEval(k, a.Repl.Cond, ctx)
				if debugEntersTappedMutate != 0 && a.Repl.Cond != 0 {
					if debugEntersTappedMutate == 1 {
						ok = true
					} else if !ok {
						// mutation: evaluate with the entering object on the
						// battlefield (gorge excludes it; the kernel cleared it
						// from occ/ctrlM above)
						s.occ.set(sl)
						s.ctrlM[p.ctrl].set(sl)
						ok = s.condEval(k, a.Repl.Cond, ctx)
						s.occ.clr(sl)
						s.ctrlM[p.ctrl].clr(sl)
					}
				}
				if ok {
					s.runReplProg(k, sl, cid, uint8(i), 0)
				}
			}
		}
	}
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if src == sl || s.genLost(k, src) {
			continue
		}
		scid := s.cardAt(src)
		if !g.has(scid, gfOthersRep) {
			continue
		}
		sp := &s.perms[src]
		cd := &k.t.Cards[scid]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplOthersEnterProgram {
				continue
			}
			s.occ.set(sl) // the filter reads the entering object
			ok := s.matchPermGen(k, a.Repl.Filter, sl, sp.ctrl, src, layerFull)
			s.occ.clr(sl)
			if ok {
				s.runReplProgAs(k, sl, scid, uint8(i), 0, src)
			}
		}
	}
	s.g.replaced = saveRep
	switch ovr {
	case entOvrTap:
		p.flags |= pfTapped // the last body applied taps it
	case entOvrUntap:
		p.flags &^= pfTapped // the last body applied untaps it
	}
	s.occ.set(sl)
	s.ctrlM[p.ctrl].set(sl)
	if g.anyEntryMerge {
		s.entryWinClose(k, sl, p)
	}
	s.refreshTy(k)
}

// runReplProg runs ability ab of card c (a replacement program) from pc
// on the entering permanent at sl, with that permanent as the source.
func (s *State) runReplProg(k *prep, sl Slot, c CardID, ab, pc uint8) {
	s.runReplProgAs(k, sl, c, ab, pc, sl)
}

// runReplProgAs runs it with the permanent at src as the source (an
// "other permanents enter" replacement). A choice stops it: the rest runs
// once the choice is answered.
func (s *State) runReplProgAs(k *prep, sl Slot, c CardID, ab, pc uint8, src Slot) {
	prog := k.t.Cards[c].Abilities[ab].Prog
	saved := s.res
	if !s.res.active {
		// a value context for the program: the source as a resolving entry
		sp := &s.perms[src]
		s.res = resCtx{e: stackEntry{kind: seAbility, obj: sp.obj, srcVer: s.objs[sp.obj].ver, card: s.cardAt(src), ab: ab, ctrl: sp.ctrl, trigObj: NoObj}}
		s.res.e.tgt = noTargets
	}
	for i := int(pc); i < len(prog); i++ {
		in := &prog[i]
		switch in.Op {
		case kop.OpBecomeCopy:
			s.becomeCopyEnter(k, sl, in)
		case kop.OpChooseColor, kop.OpChooseType:
			if s.g.pre.state == 2 && s.g.pre.obj == s.perms[sl].obj {
				s.perms[sl].chosen = s.g.pre.val // asked on the stack
				s.touchSl(sl)
				s.g.pre.state = 0
				continue
			}
			kind := akColor
			if in.Op == kop.OpChooseType {
				kind = akType
			}
			s.queueAskAt(sl, kind, c, ab, uint8(i+1))
			if !saved.active {
				s.res = saved
			}
			return
		case kop.OpChooseBranch:
			if in.C != 2 {
				s.execGen(k, in)
				break
			}
			s.siegeModeRepl(sl, c, ab, i) // face_siegemode.go
			if !saved.active {
				s.res = saved
			}
			return
		case kop.OpSkipUnless:
			i += s.replSkip(k, in) // repl_entersunless.go
		case kop.OpUnlessPaid:
			// the shock lands' as-enters pay-life election: answered from
			// preAsk, or queued here mid-resolution (repl_shockland.go)
			res, skip := s.shockAt(k, sl, c, ab, in, i)
			switch res {
			case shockNo:
				s.execGen(k, in)
			case shockQueued:
				if !saved.active {
					s.res = saved
				}
				return
			default:
				i += skip
			}
		default:
			s.execGen(k, in)
		}
	}
	if !saved.active {
		s.res = saved
	}
}

// queueAsk queues an as-enters choice for the permanent at sl (no program
// continuation).
func (s *State) queueAsk(sl Slot, kind uint8) { s.queueAskAt(sl, kind, NoCard, 0, 0) }

func (s *State) queueAskAt(sl Slot, kind uint8, c CardID, ab, pc uint8) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	if s.g.nask == 0 {
		s.g.askPrio = s.prio
	}
	a := &s.g.ask[s.g.nask]
	a.sl, a.ver, a.kind, a.card, a.ab, a.pc, a.ts = sl, s.objs[s.perms[sl].obj].ver, kind, c, ab, pc, 0
	s.g.nask++
}

// queueAskRet queues the host ask of Aura card o waiting in exile to
// return (sl NoSlot, the card's object in ab; pc set once gorge would drop
// the answer). gorge asks it at once, in the middle of the batch of
// returns, so the hosts are the permanents on the battlefield now (entry
// stamp up to s.ts): one returned later in the same batch (another
// Sheltered by Ghosts' exile) is no answer.
func (s *State) queueAskRet(o Obj) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	if s.g.nask == 0 {
		s.g.askPrio = s.prio
	}
	a := &s.g.ask[s.g.nask]
	a.sl, a.ver, a.kind, a.card, a.ab, a.pc, a.ts = NoSlot, s.objs[o].ver, akEnchant, s.objs[o].card, uint8(o), 0, s.ts
	s.g.nask++
}

// retAuraAsk poses the queued host ask a of a returning Aura (queueAskRet),
// or settles it without one: the card left exile (dropped), nothing it can
// enchant (it stays in exile, CR 303.4g) or one host (it enters attached).
func (s *State) retAuraAsk(k *prep, d *Decision, a *askRec) bool {
	o := Obj(a.ab)
	x := &s.objs[o]
	var buf [MaxOptions]uint8
	n := 0
	if x.zone == kop.ZoneExile && x.ver == a.ver {
		m := s.enchantCands(k, x.card, x.owner(), &buf)
		for i := 0; i < m; i++ {
			if s.perms[s.objs[buf[i]].slot].ts <= a.ts {
				buf[n] = buf[i]
				n++
			}
		}
	}
	if n > 1 {
		d.Player, d.Kind, d.Sub = x.owner(), DkChoose, ChGenEnchant
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
		return true
	}
	s.popAsk()
	if n == 1 {
		s.enterRetAura(k, o, s.objs[buf[0]].slot)
	}
	return false
}

// enterRetAura puts the returning Aura o onto the battlefield from exile
// attached to host (CR 303.4f).
func (s *State) enterRetAura(k *prep, o Obj, host Slot) {
	from := s.detach(o)
	sl := s.enterFromGrantedZone(k, o, s.objs[o].owner(), false, from)
	if host != NoSlot && s.occ.has(host) {
		s.attachTo(k, sl, host)
	}
}

// leaveGen runs at the end of the core's removePerm for the object o that left
// slot sl: a permanent exiled "until this leaves" comes back, at once or, in
// an open departure frame, at its end (zone_untilleavesbatch.go).
func (s *State) leaveGen(k *prep, sl Slot, o Obj) {
	if k.gen.anyFaces {
		s.faceReset(k, o) // face_multiface.go
	}
	s.clearFx(sl)
	if s.g.batchLatch&batchHeld != 0 {
		s.holdReturns(sl, o)
	} else {
		s.returnLinked(k, s.perms[sl].ctrl, o) // the controller read before a returning card takes the slot
	}
	s.refreshTy(k)
}

// returnLinked returns what the departed source o exiled until it left; srcCtrl
// is its last controller.
func (s *State) returnLinked(k *prep, srcCtrl uint8, o Obj) {
	for i := 0; i < int(s.g.nlinks); i++ {
		l := s.g.links[i]
		if l.src != o || s.objs[o].ver != l.sver+1 || k.gen.anyRemPersist && k.t.Cards[s.objs[o].card].RemPersist || s.g.linkHide>>uint(i)&1 != 0 {
			continue // (a RemPersist link is its leaves trigger's: zone_rememberpersist.go; a Hideaway link stays: kw_hideaway.go)
		}
		toHand := s.g.linkHand|s.g.linkHide|s.g.linkAeg != 0 && s.dropLinkHand(i) // act_mayask.go
		copy(s.g.links[i:], s.g.links[i+1:s.g.nlinks])
		s.g.nlinks--
		i--
		x := &s.objs[l.obj]
		if toHand {
			if x.zone == kop.ZoneExile {
				from := s.detach(l.obj)
				s.placeFromK(k, l.obj, kop.ZoneHand, false, from, false)
			}
			continue
		}
		if x.zone == kop.ZoneExile && s.res.active && s.res.ask == raNone && s.g.pre.state == 0 && s.auraWithHosts(k, l.obj) > 1 {
			// a returning Aura (Witness Protection under Banishing Light):
			// during a resolution gorge asks its host with the card still
			// in exile (CR 303.4f); it enters attached once answered
			// (raGenRetChoice; pair 5605)
			s.g.pre = preAsk{obj: l.obj, kind: akEnchant, state: preReturn}
			if k.gen.anyLinkCtr {
				s.linkRetStall(k, srcCtrl, o, l.obj)
			}
			continue
		}
		if x.zone == kop.ZoneExile && !s.res.active && s.g.nask == 0 && s.auraWithHosts(k, l.obj) > 1 {
			// outside a resolution (its source died to a state-based
			// action) gorge parks the entry with the card still in exile
			// and asks its owner the host (CR 303.4f): queued before the
			// next priority (queueAskRet)
			s.queueAskRet(l.obj)
			if k.gen.anyLinkCtr {
				s.linkRetStall(k, srcCtrl, o, l.obj)
			}
			continue
		}
		if n := s.auraWithHosts(k, l.obj); x.zone == kop.ZoneExile && (n == 1 || n > 1 && !s.res.active) {
			// a returning Aura with one permanent it can enchant enters
			// attached to it (CR 303.4f; Witness Protection under a
			// destroyed Banishing Light, pair 64152); so does one with
			// several while an ask is already pending, to its first
			// candidate: gorge parks no second entry (applyETBChoiceReplacement
			// needs no pending decision; auraEntryGate's default, in
			// gorge's battlefield order: player 0's list, then player 1's,
			// each in entry order)
			ctrN := 0
			if k.gen.anyLinkCtr {
				ctrN = s.linkRetCtrs(k, srcCtrl, o, l.obj) // before the card takes the source's slot
			}
			host := s.firstHost(k, l.obj)
			if s.g.nask > 0 && s.g.ask[0].sl == NoSlot {
				s.g.ask[0].pc = 1 // its entry record is overwritten (GORGE-FINDINGS.md D-aura-reask)
			}
			from := s.detach(l.obj)
			nsl := s.enterLinkRet(k, l.obj, x.owner(), from)
			s.attachTo(k, nsl, host)
			if ctrN > 0 {
				s.linkRetPut(k, ctrN, nsl)
			}
			continue
		}
		if x.zone == kop.ZoneExile && k.cards[x.card].types&kop.TAura != 0 && k.t.Cards[x.card].Enchant.Kind == kop.TgtObject && s.auraWithHosts(k, l.obj) == 0 {
			continue // nothing it can enchant: it stays in exile (CR 303.4g; pair 74242)
		}
		if x.zone == kop.ZoneExile {
			if kind := s.enterChoiceKind(k, x.card); kind != 0 && s.res.active && s.res.ask == raNone && s.g.pre.state == 0 {
				// during a resolution gorge asks the as-enters choice with
				// the card still in exile (Heraldic Banner, pair 544); it
				// enters once answered (runProg, raGenRetChoice)
				s.g.pre = preAsk{obj: l.obj, kind: kind, state: preReturn}
				if k.gen.anyLinkCtr {
					s.linkRetStall(k, srcCtrl, o, l.obj)
				}
				continue
			}
			ctrN := 0
			if k.gen.anyLinkCtr {
				ctrN = s.linkRetCtrs(k, srcCtrl, o, l.obj) // before the card takes the source's slot
			}
			from := s.detach(l.obj)
			nsl := s.enterLinkRet(k, l.obj, x.owner(), from)
			if ctrN > 0 {
				s.linkRetPut(k, ctrN, nsl)
			}
		}
	}
}

// firstHost is the first permanent the returning Aura card o can enchant
// in gorge's battlefield order (player 0's list, then player 1's, each in
// entry order), NoSlot when none.
func (s *State) firstHost(k *prep, o Obj) Slot {
	var buf [MaxOptions]uint8
	host := NoSlot
	n := s.enchantCands(k, s.objs[o].card, s.objs[o].owner(), &buf)
	for i := 0; i < n; i++ {
		if h := s.objs[buf[i]].slot; host == NoSlot || bfBefore(&s.perms[h], &s.perms[host]) {
			host = h
		}
	}
	return host
}

// bfBefore reports whether p comes before q in gorge's battlefield order.
func bfBefore(p, q *perm) bool {
	return p.ctrl < q.ctrl || p.ctrl == q.ctrl && p.ts < q.ts
}

// auraWithHosts is the number of objects returning Aura card o could
// enchant, or 0 when o is no Aura.
func (s *State) auraWithHosts(k *prep, o Obj) int {
	c := s.objs[o].card
	if k.cards[c].types&kop.TAura == 0 || k.t.Cards[c].Enchant.Kind != kop.TgtObject {
		return 0
	}
	var buf [MaxOptions]uint8
	return s.enchantCands(k, c, s.objs[o].owner(), &buf)
}

// newTurnGen resets the per-turn generated state (the core's beginTurn).
func (s *State) newTurnGen() {
	s.g.died = 0
	s.g.fog = false
	s.g.noPoison = 0
	if s.ndly != 0 {
		s.dropTurnLeave() // trig_leavereturn.go: "this turn" leave records end
	}
	s.g.prevCol = [3]struct {
		obj Obj
		col kop.ColorMask
	}{}
	if s.g.ctrAdd.pl|s.g.ctrAdd.n != 0 {
		s.ctrAddedReset() // ctr_addedthisturn.go
	}
	s.g.attacked = [2]uint8{}
	s.g.isCast = [2]uint8{}
	s.g.hist = [2][3]uint8{}
	clear(s.cold.castRing)
	s.g.actRing = [2][actRingN]uint8{}
	s.g.actN = [2]uint8{}
	s.g.spent = [2]uint16{}
	if k := s.k(); k.gen != nil && k.gen.anyGameOnce {
		s.keepGameOnce() // GameActivationLimit records outlive the turn (trig_params.go)
	} else {
		s.g.nused = 0
	}
	s.g.zoneTurn = objBits{}
	s.g.enteredN = 0
	s.g.enteredFull = false
	s.mSlZero(s.k(), mechExileEx)
	clear(s.cold.exMark)
	s.mustBlockNewTurn(s.k()) // combat_mustbeblocked.go
	if k := s.k(); k.gen != nil && k.gen.anyPermMark {
		s.permMarkNewTurn(k) // filter_permmarks.go
	}
	if k := s.k(); k.gen != nil && (k.gen.anyAttackedObj || k.gen.anyCombatHit) {
		s.turnTalliesNewTurn(k) // value_turntallies.go
	}
	if k := s.k(); k.gen != nil && k.gen.anyAddPhase {
		s.addPhaseNewTurn() // act_addphase.go
	}
	s.mObjZero(s.k(), mechMayhem) // the discard-this-turn marks end with the turn (CR 702.187a; gorge's TurnChange bound)
	s.g.trigTurn = 0
	s.g.dilemmas = [2]uint8{}
	if s.g.nchPick != 0 {
		s.charmPickTurn() // modal_charmpick.go: ChoiceRestriction$ ThisTurn picks end
	}
	s.g.crime = crimeKeep(s.g.crime)
	if s.g.ndp != 0 {
		s.dropDelayedSpell() // ThisTurn$ SpellCast registrations (trig_delayedspell.go)
		s.dropDelayedDies()  // ThisTurn$ dies registrations (trig_delayeddies.go)
	}
}

// touchBits marks every object in b changed. A generated-state clear that
// changes a view of an object without writing the object itself must call
// it, so a reader caching per object (dirac/feat) rebuilds that view.
func (s *State) touchBits(b *objBits) {
	for i, w := range b {
		for w != 0 {
			s.touch(Obj(i*64 + bits.TrailingZeros64(w)))
			w &= w - 1
		}
	}
}

// cleanupGen ends "until end of turn" generated effects (the core's
// finishCleanup).
func (s *State) cleanupGen(k *prep) {
	// A granted flashback is part of a graveyard card's view although no
	// object field names it: mark every card whose grant this clear ends
	// before dropping the bitsets.
	s.fbClearAll(k)
	if s.g.nfx > 0 {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.kind == fxOverride && r.r.eot() })
		if k.gen.anyNTOv {
			s.dropNextTurnOv() // Duration$ UntilYourNextTurn (static_control.go)
		}
	}
	if k.gen.anyEOTEmb {
		s.dropEOTEmblems(k)
	}
	if k.gen.anyExHaste {
		s.exhClear() // static_cantblockexcept.go
	}
	if k.gen.anyColGrant {
		s.cgClear() // static_colourgrant.go
	}
	if k.gen.anyNTEmb {
		s.dropNextTurnEmblems(k) // Duration$ UntilYourNextTurn (static_effectdelivered.go)
	}
	n := 0
	for i := 0; i < int(s.g.nmay); i++ {
		r := s.g.may[i]
		keep := r.dur() == kop.DurYourNextTurn && !(s.active == r.ctrl && s.turn > r.turn) ||
			r.until == kop.DurWarp && s.warpRecastLive(r) ||
			r.until == kop.DurAirbend && s.warpRecastLive(r) ||
			r.until == kop.DurPlot && s.plotLive(r) ||
			r.until == kop.DurPrepared && s.preparedSrc(r) ||
			r.dur() == kop.DurWhileSrcMay && s.mayWhileSrcKeep(r) || // cast_whilesource.go
			r.dur() == kop.DurWhileValid && s.plotLive(r) ||
			r.until == kop.DurPermanent && s.plotLive(r) // a while-exiled permission (cast_whileexiled.go)
		if keep {
			r.until &^= mayUntilUsed // a grant's cap counts per turn (cast_mayplaylimit.go)
			s.g.may[n] = r
			n++
		}
	}
	s.g.nmay = uint8(n)
	s.refreshTy(k)
}

// fireDelayedGen puts the delayed programs on the stack at the beginning of
// the end step (as gorge's delayed triggers).
func (s *State) fireDelayedGen(k *prep) {
	if k.gen.anyDelayUp {
		s.fireDelayedWhen(k, 0)
		return
	}
	n := s.g.ndp
	// ndp drops after the loop, not before: until each program's LKI index
	// is copied into its pending trigger, the programs hold their records
	// (an enqueue's snapshot sweeps the pool, zone_lki.go)
	for i := 0; i < int(n); i++ {
		d := s.g.dp[i]
		m := s.npend
		ro, remVer, remSet := s.delayRem(k, &d)
		var amt int16
		if d.n&dpCountMask == 2 {
			amt = s.dpSecond(&d) // Mobilize 2's second token (kw_mobilize.go)
		}
		s.enqueue(d.src, s.objs[d.src].ver, d.card, d.ab, d.ctrl, ro, d.ctrl, amt)
		if s.npend > m {
			s.pend[m].li = s.g.dpLKI[i]
			s.pend[m].remVer, s.pend[m].remSet = remVer, remSet
			if d.n&dpMany != 0 {
				s.dpManyCarry(&d, &s.pend[m]) // trig_delayedmany.go
			}
		}
	}
	s.g.ndp = 0
}

// delayRem is what delayed program d remembers as it fires, with the version it
// was registered at and whether a read must match it. An object that changed
// zones since is a new one, with nothing to return (exiled from the graveyard,
// CR 400.7), but a copy of it reads last known information (CR 608.2h).
func (s *State) delayRem(k *prep, d *dprog) (Obj, uint8, bool) {
	ro := d.objs[0]
	switch {
	case ro == NoObj:
	case delayCopies(k, d.card, d.ab):
		return ro, d.vers[0], false
	case s.objs[ro].ver == d.vers[0]:
		return ro, d.vers[0], true
	}
	return NoObj, d.vers[0], false
}

// delayCopies reports whether a card's delayed ability copies its remembered
// object.
func delayCopies(k *prep, card CardID, ab uint8) bool {
	for i := range k.t.Cards[card].Abilities[ab].Prog {
		in := &k.t.Cards[card].Abilities[ab].Prog[i]
		if (in.Op == kop.OpCopyAsToken || in.Op == kop.OpCopyAsTokenEx) && in.Obj == kop.RefDelayedRemembered {
			return true
		}
	}
	return false
}

// stepGen runs before each of the core's control states: it poses a pending
// generated-table decision or starts a queued action. It returns true with
// d filled when a decision is posed.
func (s *State) stepGen(k *prep, d *Decision) bool {
	if s.ctl == ctlGenAsk {
		return s.genAskOptions(k, d)
	}
	s.refreshTy(k)
	if s.g.stalled != 0 {
		s.stall(s.g.stalled) // a pool was full (capFull): more simultaneous triggers than MaxPending, ...
		d.Kind = DkNone
		return true
	}
	if s.g.trigTurn > maxTrigTurn && !s.res.active {
		// a mandatory loop that never ends the game (Marauding Blight-Priest
		// with Bloodthirsty Conqueror against "can't lose"): CR 104.4b draw
		s.stall(StallLoop)
		d.Kind = DkNone
		return true
	}
	if (s.ctl == ctlPriority || s.ctl == ctlPlaceTriggers) && s.sp >= MaxStack && !s.res.active {
		s.stall(StallStack) // nothing more fits on the stack: the kernel's capacity
		d.Kind = DkNone
		return true
	}
	if s.ctl != ctlPriority || s.res.active {
		return false
	}
	if s.g.endTurn {
		s.clearCombat() // CR 723.1b: every creature is removed from combat
		// a state-based action still due is performed before the step: inside
		// it, CR 514.3a would give priority for it
		if s.sba(k) || s.ctl != ctlPriority {
			return false
		}
		if s.g.nask == 0 {
			s.g.endTurn = false
			s.endStep(kop.StepCleanup)
			return false
		}
	}
	if s.g.ro.n > 0 {
		// a noncombat damage event parked on gorge's replacement-order ask
		// (parkNC): asked once the resolution is done, before priority
		s.g.ro.nca = true
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		return s.genAskOptions(k, d)
	}
	if s.g.nask > 0 || s.g.leyOn {
		s.g.ctlRet = s.ctl
		s.ctl = ctlGenAsk
		if s.g.nask > 0 && s.g.priSave == 0 {
			s.g.priSave = 0x80 | s.prio
			s.prio = s.g.askPrio
		}
		return s.genAskOptions(k, d)
	}
	if s.g.nfq > 0 && !s.g.hold && int(s.sp) < MaxStack && s.holdable() {
		// gorge casts them while the ability (Etali's trigger) or spell
		// (Walk with the Ancestors' discover) is still resolving: it stays
		// on the stack under them until the last cast
		h := s.res.e
		if h.kind != seSpell {
			h.flags |= sfHold // a spell's bit is sfPromised: its hold is res.e (spellHeld)
		}
		s.push(h)
		s.g.hold = true
		if s.g.priSave == 0 {
			// the casts are made mid-resolution: the holder is unchanged
			s.g.priSave = 0x80 | s.prio
			s.prio = s.g.holdPrio
		}
	}
	if s.g.nfq == 0 && s.res.ask == raGenFreeParked {
		s.resumeFreeCast(k) // the casts are made: the parked tail runs (act_discoverfreecast.go)
		return false
	}
	if s.g.nfq == 0 && s.g.hold {
		s.dropHold(k)
	}
	for s.g.nfq > 0 {
		o := s.g.freeQ[0]
		copy(s.g.freeQ[:], s.g.freeQ[1:s.g.nfq])
		s.g.nfq--
		c := s.objs[o].card
		// freeP: bit 0 caster, bits 1-3 zone, bit 4 ReplaceGraveyard, bit 5 CopyCard
		ctrl := s.g.freeP & 1
		if s.g.freeP&32 != 0 {
			// gorge casts the copy whatever became of the original
			if !s.spellTargetsFeasible(k, c, ctrl) || !s.spellCostsFeasible(k, c, ctrl, o) || !s.beginCopyCast(k, o, ctrl) {
				if s.over {
					return false
				}
				continue
			}
			s.bld().e.flags |= sfFree
			return false
		}
		if s.objs[o].zone != kop.Zone((s.g.freeP>>1)&7) || !s.spellTargetsFeasible(k, c, ctrl) || !s.spellCostsFeasible(k, c, ctrl, o) {
			continue // no legal target any more: the card stays in its zone
		}
		holder := s.prio
		s.prio = ctrl // the caster (beginCast reads it)
		s.beginCast(k, o, castFree)
		s.prio = holder
		return false
	}
	if s.res.ask == raGenFreeParked {
		s.resumeFreeCast(k) // the rest were refused: the parked tail runs
		return false
	}
	if s.g.hold {
		s.dropHold(k) // the rest were refused (no legal target): the ability is done
	}
	return false
}

func (s *State) genAskOptions(k *prep, d *Decision) bool {
	if k.gen.anyBattle && s.g.nmay > 0 && s.defeatPosedOptions(d) {
		return true
	}
	if s.entryAsked(k) {
		s.entryOrderOptions(k, d)
		return true
	}
	if s.g.ro.n > 0 && (s.g.ro.on || s.g.ro.nca) {
		s.replOrderOptions(d)
		return true
	}
	if s.g.pre.state == 1 {
		d.Kind = DkChoose
		if s.g.pre.land {
			d.Player = s.prio // the land's player still holds priority
		} else {
			d.Player = s.stack[s.sp-1].ctrl
		}
		s.askOptions(k, d, s.g.pre.kind)
		return true
	}
	for s.g.leyOn {
		// one yes/no per Leyline in hand, hand order, starting player first
		// (gorge's opening_yes / opening_no)
		if o := s.leyCard(k, s.g.leyP, s.g.ley[1]); o != NoObj {
			d.Kind, d.Sub, d.Player = DkChoose, ChGenLeyline, s.g.leyP
			d.add(MakeAction(AkChoose, uint8(o), 0, 1))
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
			return true
		}
		s.nextLeyline(k)
		if !s.g.leyOn {
			return false
		}
	}
	for s.g.nask > 0 {
		a := &s.g.ask[0]
		if a.sl == NoSlot {
			if s.retAuraAsk(k, d, a) {
				return true
			}
			continue
		}
		ob := &s.objs[s.perms[a.sl].obj]
		if !s.occ.has(a.sl) || ob.ver != a.ver {
			s.popAsk()
			continue
		}
		d.Player = s.perms[a.sl].ctrl
		d.Kind = DkChoose
		s.askOptions(k, d, a.kind)
		return true
	}
	s.ctl = s.g.ctlRet
	if s.g.priSave != 0 {
		s.prio, s.g.priSave = s.g.priSave&1, 0
	}
	return false
}

func (s *State) popAsk() {
	copy(s.g.ask[:], s.g.ask[1:s.g.nask])
	s.g.nask--
}

func (s *State) applyGen(k *prep, a Action) {
	if k.gen.anyBattle && s.g.nmay > 0 && s.applyDefeat(k, a) {
		return
	}
	if s.entryAsked(k) {
		s.applyEntryLand(k, a)
		return
	}
	if s.g.ro.n > 0 && (s.g.ro.on || s.g.ro.nca) {
		s.applyReplOrder(a)
		return
	}
	if s.g.pre.state == 1 {
		s.g.pre.val = a.Mode()
		if s.g.pre.kind == akClone {
			s.g.pre.val = cloneAnswer(a)
		}
		if s.g.pre.kind == akType {
			s.g.pre.val = typeVocabSub(k, int(a.Target())<<8|int(a.Mode()))
		}
		s.g.pre.state = 2
		s.ctl = ctlPriority
		if s.g.pre.land {
			s.playLandNow(k, s.g.pre.obj, s.prio) // the land enters with the choice made
		} else {
			s.resolveTop(k) // the permanent enters with the choice made
		}
		s.g.pre = preAsk{}
		return
	}
	if s.g.leyOn {
		if a.Mode() == 1 {
			o := Obj(a.Source())
			from := s.detach(o)
			s.enter(k, o, s.g.leyP, false, from)
		} else {
			s.g.ley[1]++ // declined: it stays in hand, ask the next one
		}
		return
	}
	s.answerQueuedAsk(k, a)
	if s.g.nask == 0 {
		s.ctl = s.g.ctlRet
		if s.g.priSave != 0 {
			s.prio, s.g.priSave = s.g.priSave&1, 0
		}
	}
}

// answerQueuedAsk applies answer a to the first queued as-enters choice
// and runs the rest of its enters program.
func (s *State) answerQueuedAsk(k *prep, a Action) {
	ask := s.g.ask[0]
	s.popAsk()
	if ask.sl == NoSlot {
		// the host answered (retAuraAsk); gorge drops the answer when
		// another Aura entered while it was asked (pc set) and attaches its
		// first candidate (GORGE-FINDINGS.md D-aura-reask)
		host := s.objs[a.Source()].slot
		if ask.pc != 0 {
			host = s.firstHost(k, Obj(ask.ab))
		}
		s.enterRetAura(k, Obj(ask.ab), host)
		return
	}
	p := &s.perms[ask.sl]
	s.touchSl(ask.sl)
	if ask.kind == akPayLife || ask.kind == akReveal || ask.kind == akRevPick {
		if ask.kind == akPayLife {
			s.shockAnswer(k, ask, a) // repl_shockland.go
		} else if !s.revealAnswer(k, ask, a) { // repl_revealland.go
			return
		}
		if ask.ts&askEntryTapped != 0 {
			s.perms[ask.sl].flags |= pfTapped
		}
		if ask.ts&askLandPlay != 0 {
			// a played land's election: the answer completes the play
			s.pl[s.perms[ask.sl].ctrl].landsPlayed++
		}
		return
	}
	if ask.kind == akMode {
		s.siegeModeStore(ask.sl, a.Mode())
	} else if ask.kind == akColor {
		p.chosen = a.Mode()
	} else {
		v := int(a.Target())<<8 | int(a.Mode())
		p.chosen = typeVocabSub(k, v)
	}
	if ask.card != NoCard {
		saveRep := s.g.replaced
		s.g.replaced = p.obj
		s.runReplProg(k, ask.sl, ask.card, ask.ab, ask.pc)
		s.g.replaced = saveRep
	}
}

// leylineStart begins the opening-hand Leyline asks (gorge's
// MayEffectFromOpeningHand) before the first turn; false when there is
// nothing to ask.
func (s *State) leylineStart(k *prep) bool {
	if k.gen == nil || !k.gen.leyline || s.g.leyDn {
		return false
	}
	s.g.leyDn = true
	if !s.hasLeyline(k, 0) && !s.hasLeyline(k, 1) {
		return false
	}
	s.g.leyP = s.startPl
	s.g.ley = [2]uint8{}
	s.g.leyOn = true
	s.g.ctlRet = ctlStepBegin
	s.ctl = ctlGenAsk
	return true
}

func (s *State) hasLeyline(k *prep, p uint8) bool {
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if k.t.Cards[s.objs[pl.hand[i]].card].Leyline {
			return true
		}
	}
	return false
}

// leyCard is player p's Leyline in hand after skipping the first skip
// (the declined ones), NoObj when none is left.
func (s *State) leyCard(k *prep, p, skip uint8) Obj {
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if o := pl.hand[i]; k.t.Cards[s.objs[o].card].Leyline {
			if skip == 0 {
				return o
			}
			skip--
		}
	}
	return NoObj
}

// nextLeyline moves the Leyline asks to the next player, or starts the
// game. ley[0] is 1 once the starting player is done, ley[1] the current
// player's declined Leylines.
func (s *State) nextLeyline(k *prep) {
	if s.g.ley[0] == 0 {
		s.g.ley = [2]uint8{1, 0}
		s.g.leyP = 1 - s.startPl
		return
	}
	s.g.leyOn = false
	s.beginTurn(s.startPl)
}

// tappedMask is player p's tapped permanents.
func (s *State) tappedMask(p uint8) slotMask {
	var m slotMask
	for q := s.ctrlM[p]; q.any(); q = q.rest() {
		sl := q.first()
		if s.perms[sl].flags&pfTapped != 0 {
			m = m.with(sl)
		}
	}
	return m
}

// askOptions fills an as-enters colour (W U B R G as mode 0..4) or creature
// type (gorge's vocabulary index as target<<8 | mode) choice.
func (s *State) askOptions(k *prep, d *Decision, kind uint8) {
	if kind == akClone {
		s.cloneOptions(k, d)
		return
	}
	if kind == akPayLife {
		s.shockOptions(k, d) // repl_shockland.go
		return
	}
	if kind == akReveal || kind == akRevPick {
		s.revealOptions(k, d, kind) // repl_revealland.go
		return
	}
	if kind == akMode {
		siegeModeOptions(d) // face_siegemode.go
		return
	}
	if kind == akEnchant {
		d.Sub = ChGenEnchant
		o := s.g.pre.obj
		var buf [MaxOptions]uint8
		n := s.enchantCands(k, s.objs[o].card, s.objs[o].owner(), &buf)
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, buf[i], 0, 0))
		}
		return
	}
	if kind == akColor {
		d.Sub = ChGenColor
		ex := s.colorExclude(k) // act_mayask.go
		for c := uint8(0); c < 5; c++ {
			if ex&(1<<c) == 0 {
				d.add(MakeAction(AkChoose, 0, 0, c))
			}
		}
		return
	}
	d.Sub = ChGenType
	if k.gen.anyTypeValid && s.typeOptions(k, d) { // ValidTypes$ (act_choosetype.go)
		return
	}
	for v := range k.t.TypeVocab {
		d.add(MakeAction(AkChoose, 0, uint8(v>>8), uint8(v)))
	}
}

// dropHold removes the held entry once the free casts are made (a held
// spell then leaves the stack: spellHoldLeaves).
func (s *State) dropHold(k *prep) {
	spell := s.res.e.kind == seSpell
	if i := s.holdIndex(); i >= 0 {
		copy(s.stack[i:s.sp], s.stack[i+1:s.sp])
		s.sp--
	}
	if spell {
		s.spellHoldLeaves(k)
	}
	s.g.hold = false
	if s.g.priSave != 0 {
		s.prio, s.g.priSave = s.g.priSave&1, 0
	}
}
