package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Resolution of the top of the stack and the kop interpreter.

// targetStillLegal rechecks target i of e at resolution (CR 608.2b).
func (s *State) targetStillLegal(k *prep, e *stackEntry, i int) bool {
	code := e.tgt[i]
	if code == NoTarget {
		return false
	}
	if isPlayerOrNone(code) {
		if pp := CodeSeat(code); s.g.on && pp != e.ctrl && s.playerHexproof(s.k(), pp) {
			return false
		}
		return !s.pl[CodeSeat(code)].lost
	}
	if isStackTarget(code) {
		return s.stackTgtStillLegal(e, i) // tgt_stackability.go
	}
	if s.objs[code].ver != e.tver[i] {
		return false
	}
	t := tspecE(s.k(), e, i)
	if t == nil {
		return false
	}
	if t.Kind == kop.TgtSpellTgting {
		return s.spellTgtingStill(e, code) // tgt_countershapes.go
	}
	src := NoSlot
	if e.kind != seSpell {
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
			src = ob.slot
		}
	}
	return s.targetOK(k, t, e.ctrl, src, e.obj, code)
}

// resolveTop resolves the top of the stack. The entry stays on the stack
// until its resolution finishes (gorge keeps it there through
// mid-resolution choices); finishResolve pops it.
func (s *State) resolveTop(k *prep) {
	e := s.stack[s.sp-1]
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.trigGone = e.kind == seTrigger && e.li != 0 && e.trigObj != NoObj && s.objs[e.trigObj].ver > s.lk(e.li).ver+1
	if k.gen != nil {
		s.g.nimp = 0
		s.g.nrem, s.g.remCnt, s.g.tli = 0, 0, 0 // a resolution remembers afresh (gorge's per-resolution Ctx)
		s.g.stkCt = 0                           // tgt_stackability.go
		if k.anyStateCtr && e.kind == seTrigger && e.ab < abCascade &&
			k.t.Cards[e.card].Abilities[e.ab].Trig.Flags4&kop.Tf4StateCtr != 0 {
			s.remember(e.obj) // a state trigger's Remembered is its source alone (gorge stateTriggerRemembered, CR 603.8)
		}
		if k.gen.anyRemPersist && remPersistSeeds(k, &e) {
			s.remPersistSeed(k) // its source's kept set (zone_rememberpersist.go)
		}
		// a damage sequence belongs to the resolution that started it: a new
		// resolution drops any left over by an aborted one (repl_gen.go); the
		// looked-at cards are the resolution's too (arm C of scr)
		s.scrResEnter()
		if k.gen.anyTgtSpent {
			s.snapTgtSpent(&e) // the target's spend, before a counter removes its entry
		}
		if k.gen.anyCastSpent && DebugOpusLive {
			s.res.e.trigAmt = s.liveSpentAmt(k, &e) // TestOpusMutation only
		}
	}
	if e.mode&modeBestowed != 0 && e.bestowedEntry() {
		s.bestowResolve(k, &e) // CR 702.103e (kw_bestow.go)
	}
	if k.gen != nil && e.kind == seSpell && isObjTarget(e.tgt[0]) && e.li == 0 {
		// the target's last known P/T (Condemn); a sacrificed cost's LKI,
		// already held, is kept (RefSacrificed: Kazuul's Fury deals the
		// sacrificed creature's power, not its target's)
		s.res.e.li = s.snapLKI(Obj(e.tgt[0]))
	}
	// fizzle: every chosen target illegal
	if e.nt > 0 {
		any := false
		chosen := false
		for i := 0; i < int(e.nt); i++ {
			if e.tgt[i] == NoTarget {
				continue
			}
			chosen = true
			if s.targetStillLegal(k, &e, i) {
				any = true
			} else {
				s.res.bad |= 1 << i // illegal: ops on it do nothing
			}
		}
		if k.gen != nil && e.xi != 0 {
			s.res.xbad = s.xtIllegal(k, &e) // the damager extras as the resolution starts (extop.go)
		}
		if chosen && !any && k.gen != nil {
			// a multi-object slot's other objects (Chandra's divided -X with
			// Chandra herself as the first target) keep it from fizzling
			for j := 0; j < int(s.xtOf(&e).n); j++ {
				if s.xtLegal(k, &e, j) {
					any = true
					break
				}
			}
		}
		if chosen && !any {
			s.finishResolve(k, true)
			return
		}
	}
	switch e.kind {
	case seSpell:
		if k.cards[e.card].types&(kop.TInstant|kop.TSorcery) == 0 {
			// permanent spell
			if k.gen != nil && !e.faceDown() && s.preEnterAsk(k, &e) {
				return
			}
			if k.gen != nil && !e.faceDown() && s.cloneEntryOrderPark(k, e.obj, e.ctrl) {
				return // the CR 616.1 order between the copy and another entry replacement (repl_entryclone.go)
			}
			// the CR 616.1 order between the effect's counters and the
			// entrant's own body (repl_etborder.go), gated on the stamped
			// source first: one load of the copied entry when no rider pends
			if k.gen != nil && !e.faceDown() && e.etbSrc != 0 && s.etbEntryOrderPark(k, e.obj, e.ctrl) {
				return
			}
			if k.gen != nil && s.occ.full() {
				s.stall(StallBoard) // the battlefield is full: the capacity draw
				return
			}
			s.sp--
			s.touch(e.obj)
			ob := &s.objs[e.obj]
			ob.zone = kop.ZoneNone
			var sl Slot
			if e.faceDown() {
				sl = s.enterFaceDown(k, e.obj, e.ctrl, e.morphFamily())
			} else {
				if k.gen != nil && k.gen.anyPermCopy {
					s.permCopyEnters(e.obj) // CR 111.13: a copy becomes a token
				}
				if e.mode&modeBestowed != 0 && e.bestowedEntry() {
					sl = s.enterAttached(k, &e) // kw_bestow.go
				} else {
					if k.gen != nil && k.gen.anyImpending {
						s.impendEnter(k, &e) // N time counters (kw_impending.go)
					}
					if k.cards[e.card].types&kop.TAura != 0 {
						sl = s.enterAttached(k, &e)
					} else {
						sl = s.enter(k, e.obj, e.ctrl, false, kop.ZoneStack)
					}
				}
			}
			if k.gen != nil && k.gen.anyManaHaste {
				s.manaHasteEnter(k, sl, &e) // mana_spentrider.go
			}
			if k.gen != nil && k.gen.anyEvoke {
				s.evokeEnter(k, &e) // the Adamant bind and the evoke sacrifice (kw_evoke.go)
			}
			if k.gen != nil && k.gen.anyDash && e.dashed(k) {
				s.dashEnter(sl, &e) // haste and the end-step return to hand (kw_dash.go)
			}
			if k.gen != nil && k.gen.anyWarp && e.mode&modeWarped != 0 {
				s.registerWarpExile(e.obj, e.ctrl) // kw_warp.go
			}
			if k.gen != nil && k.gen.anySneak && e.trigPl == trigPlSneak {
				s.sneakEnter(k, sl, &e) // CR 702.190b (kw_sneak.go)
			}
			if k.gen != nil && s.cloneEntryLatePark(sl) {
				return // gorge's second CR 616.1 ask, with the permanent in and the copy still to apply (repl_entryclone.go)
			}
			s.res.active = false
			s.afterResolve()
			return
		}
	case seTrigger, seAbility:
		if e.ab == abProwess {
			ob := &s.objs[e.obj]
			if ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
				s.touch(e.obj)
				op, ot := s.eotPT(ob.slot)
				s.setEotPT(ob.slot, op+1, ot+1)
			}
			s.finishResolve(k, false)
			return
		}
		if e.ab == abCascade || e.ab == abStorm {
			s.runProg(k)
			return
		}
		if e.ab == abWard {
			s.resolveWard(k)
			return
		}
		if e.ab == abMadness {
			s.resolveMadness(k) // kw_madness.go
			return
		}
		if e.ab == abMiracle {
			s.resolveMiracle(k) // kw_miracle.go
			return
		}
		if e.ab == abDelayed {
			if e.trigAmt == initTrigAmt && e.obj == NoObj {
				if s.resolveVentureTrig(k) { // act_initiative.go
					s.finishResolve(k, false)
				}
				return
			}
			s.resolveDelayed(k, &e)
			if s.res.ask != raNone {
				return // casualty's copy elects (copy_gen.go); copyResume finishes it
			}
			if k.gen != nil && s.parkReturnAsk() {
				return // the delayed action's sacrifice returned a card whose as-enters choice is asked with it still in exile (zone_untilleavesbatch.go)
			}
			s.finishResolve(k, false)
			return
		}
		if e.kind == seTrigger && e.trigAmt == dpTrackAmt && s.objs[e.obj].ver != e.srcVer {
			s.finishResolve(k, false) // a tracked promise whose source is a new object: nothing (CR 400.7)
			return
		}
		if k.gen != nil && !s.trigResolveOK(k) {
			s.finishResolve(k, false)
			return
		}
		a := &k.t.Cards[e.card].Abilities[e.ab]
		if e.kind == seAbility && e.ab < abCascade && a.AFlags&kop.AfResolveCount != 0 {
			s.countResolve(k, &e) // an activated ability's tally, past the fizzle check (gorge's Resolve event)
		}
		if a.Trig.Flags2&kop.Tf2ResolvedLimit != 0 && e.kind == seTrigger && s.rlSpent(e.obj) {
			s.finishResolve(k, false) // a batched sibling already spent the limit: no ask, no effect, no count (gorge resolvedLimitSpent)
			return
		}
		if a.Trig.Flags2&kop.Tf2ResolvedLimit != 0 && e.kind == seTrigger && a.Trig.Flags&kop.TfOptional == 0 {
			s.countRL(&e) // a mandatory limited line counts as it resolves (act_support.go)
		}
		if a.Trig.Flags&kop.TfOptional != 0 && e.kind == seTrigger && (k.gen == nil || !k.gen.anyCloneCopy || !s.cloneTrigMoved(k, &e)) {
			s.res.ask = raOptional
			s.res.askP = optTrigAsker(&e, a.Trig.Flags4) // trig_optplayer.go
			s.ctl = ctlResolve
			return
		}
		if e.kind == seAbility && a.NModes != 0 && s.modalActAsk(k, a) {
			return // the modes are chosen as it resolves (modal_activated.go)
		}
	}
	s.runProg(k)
}

// spellRestExiles reports whether a spell that resolved or fizzled rests in
// exile rather than its owner's graveyard: a copy of a spell ceases to exist
// as it leaves the stack (CR 707.10a), which gorge's spellRestZone and
// spellFizzleZone spell with an exile move, and the flashback family does the
// same (exilesOnLeave). A COUNTERED copy does not use this: gorge's Counter
// sends it to the zone its card would go to, and the state-based check ceases
// it there (counterAt).
func (s *State) spellRestExiles(e *stackEntry) bool {
	return e.exilesOnLeave() || s.objs[e.obj].of&ofCopy != 0
}

// finishResolve completes a resolution: a resolving instant/sorcery goes
// to its owner's graveyard (exile when flashed back, and a copy always
// leaves the stack for exile: spellRestExiles).
func (s *State) finishResolve(k *prep, fizzled bool) {
	if len(s.cold.parked) != 0 {
		if s.parkedWaiting() {
			s.res.ask, s.ctl = raParkFinish, ctlExileOrder // a move made outside the program loop
			return
		}
		if s.parkSpellExit(k, fizzled) {
			return
		}
	}
	if s.finishParkedReturn(k) {
		s.ctl = ctlResolve // another held return parks its own ask (zone_untilleavesbatch.go)
		return
	}
	s.sp-- // the resolving entry leaves the stack
	if k.gen != nil && k.gen.anyBattle && s.g.nmay > 0 {
		// gorge's next step() head runs startDefeatedCast after a resolution:
		// a battle defeated earlier this step is asked before the next
		// priority round.
		s.wakeDefeat()
	}
	e := &s.res.e
	if k.gen != nil && k.gen.anyRemPersist && remPersistKeeps(k, e) {
		s.remPersistKeep(e) // zone_rememberpersist.go
	}
	if e.kind == seSpell {
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneStack && s.g.nfq > 0 && !fizzled && k.gen != nil {
			// its free casts are made with it still on the stack (the
			// hold, trig_xpaid.go): it leaves after them
		} else if ob.zone == kop.ZoneStack {
			s.touch(e.obj)
			ob.zone = kop.ZoneNone
			z := kop.ZoneGraveyard
			if s.spellRestExiles(e) || e.markExiles(fizzled) { // markExiles: repl_stackexile.go
				z = kop.ZoneExile
			}
			if k.gen != nil && k.gen.anyFaces {
				z = s.stackExitFace(k, e.obj, z, !fizzled) // face_castface.go
			}
			if k.gen != nil && k.gen.anyParadigm && !fizzled {
				z = s.paradigmRest(k, e.obj, z) // kw_paradigm.go
			}
			if z == kop.ZoneLibrary {
				s.omenToLibrary(e.obj) // face_omen.go
			} else {
				s.placeFromK(k, e.obj, z, false, kop.ZoneStack, false)
				if !fizzled {
					s.markedPlot(k, e) // repl_stackexile.go
				}
			}
		}
	}
	if k.gen != nil && k.gen.anyTgtSpent {
		s.g.tgtSpent = 0 // the snapshot is the resolution's
	}
	s.res.active = false
	if k.anyStateCtr {
		// any resolution's finish may have emptied the stack of a state
		// trigger's instance -- its own, or one a countering resolution
		// removed before going on to remove the source (gorge re-checks at
		// the next event, which is at or after the resolution's end;
		// trig_statectr.go)
		s.stateCtrRefire(k)
	}
	s.afterResolve()
}

func (s *State) resolveWard(k *prep) {
	e := &s.res.e
	if wardIsGrantedLife(e.trigAmt) {
		// a granted ward--pay N life: N sits in the low byte beside the grant
		// bits; the printed life ward's path charges it
		e.trigAmt = -int16(e.trigAmt & 0xff)
		s.resolveWardLife(k)
		return
	}
	if wardIsGranted(e.trigAmt) {
		e.trigAmt &^= wardGrantedBits
	}
	if e.trigAmt < 0 {
		s.resolveWardLife(k)
		return
	}
	if e.trigAmt&(wardMLBit|wardPoisBit) == wardPoisBit {
		s.resolveWardPoison(k) // cost_restokens.go
		return
	}
	if e.trigAmt&wardPowBit != 0 {
		s.resolveWardPower(k) // kw_wardcost.go
		return
	}
	if e.trigAmt&wardAltBits == wardAltBits {
		s.resolveWardAlt(k) // kw_wardcost.go
		return
	}
	if e.trigAmt&(wardMLBit|wardWBBit) == wardWBBit {
		s.resolveWardWaterbend(k) // cost_unlesswaterbend.go
		return
	}
	if e.trigAmt&wardEvBit != 0 {
		s.resolveWardEvidence(k) // kw_wardcost.go
		return
	}
	if e.trigAmt&wardSacBit != 0 {
		s.resolveWardSac(k) // kw_wardcost.go
		return
	}
	if e.trigAmt&wardDiscBit != 0 {
		s.resolveWardDiscard(k) // kw_wardcost.go
		return
	}
	if e.trigAmt&wardMLBit != 0 {
		s.resolveWardManaLife(k) // cost_unlesstapward.go
		return
	}
	// the warded spell/ability: e.trigObj is its object, e.trigPl its controller
	idx := s.wardedIndex(e)
	if idx < 0 {
		s.finishResolve(k, false)
		return
	}
	var av avail
	s.availWindowFor(k, e.trigPl, &av)
	c := kop.ManaCost{Generic: uint8(e.trigAmt)}
	if av.affords(&c, 0, 0) || k.gen != nil {
		s.res.ask = raGeneric // pay (1) or not (0)
		s.res.askP = e.trigPl
		s.res.left = 1
		s.ctl = ctlResolve
		return
	}
	s.wardCounterAt(k, idx)
	s.finishResolve(k, false)
}

// wardedIndex is the stack entry a ward trigger e answers (-1 = gone): the spell, or
// the ability of that source that targets the ward's permanent. A source's other
// ability above it is not the one that targeted (Koth of the Homestead's two
// landfall triggers).
func (s *State) wardedIndex(e *stackEntry) int {
	i := s.stackIndexOf(e.trigObj)
	if i < 0 || s.stack[i].kind == seSpell {
		return i
	}
	for j := i; j >= 0; j-- {
		se := &s.stack[j]
		if se.obj != e.trigObj || se.kind == seSpell || e.tver[0] != 0 && se.sid != e.tver[0] {
			continue
		}
		var codes [kop.MaxTargets + maxXTgt]uint8
		n := s.chainTargets(se, &codes)
		for _, c := range codes[:n] {
			if c == uint8(e.obj) {
				return j
			}
		}
	}
	return -1
}

// stackIndexOf is o's stack entry, top down (-1 = none).
// An object on the stack is a spell, and its entry is the spell's: a
// self cast trigger's entry carries the same object (its source, the
// spell; selfcast) and sits above it, so it is passed over -- "counter
// target spell" at a Hydroid Krasis, Dash Hopes countering itself, a ward
// trigger's warded spell all mean the spell (kfix-cast).
func (s *State) stackIndexOf(o Obj) int {
	onStack := int(o) < len(s.objs) && s.objs[o].zone == kop.ZoneStack
	for i := int(s.sp) - 1; i >= 0; i-- {
		if s.stack[i].obj == o && (!onStack || s.stack[i].kind == seSpell) {
			return i
		}
	}
	return -1
}

// counterAt removes stack entry i (a countered spell goes to the graveyard).
func (s *State) counterAt(k *prep, i int) {
	e := s.stack[i]
	copy(s.stack[i:s.sp-1], s.stack[i+1:s.sp])
	s.sp--
	if e.kind == seSpell {
		s.touch(e.obj)
		ob := &s.objs[e.obj]
		ob.zone = kop.ZoneNone
		// gorge's Counter effect (effects/resolve.go effCounter) rewrites the
		// destination with state.ExilesInsteadOfGraveyard: the flashback
		// family and a ReplaceGraveyard$ Exile free cast (pin e276bf2b3; the
		// earlier pins read ExilesLeavingStack only, so a countered free cast
		// reached the graveyard).
		z := kop.ZoneGraveyard
		if e.exilesOnLeave() {
			z = kop.ZoneExile
		}
		if k.gen != nil && k.gen.anyFaces {
			z = s.stackExitFace(k, e.obj, z, false) // face_castface.go
		}
		s.placeFromK(k, e.obj, z, false, kop.ZoneStack, false)
	}
}

// --- interpreter ---

// prog is the resolving entry's program. A kernel-native entry has no
// Abilities row and, cascade aside, no program: its ward payment or delayed
// sacrifice reaches shared code that asks what the resolution is executing.
func (s *State) prog(k *prep) []kop.Instr {
	e := &s.res.e
	if e.ab >= abCascade {
		if e.ab == abCascade {
			return cascadeProg[:]
		}
		if e.ab == abStorm {
			return stormProg[:]
		}
		return nil
	}
	return k.t.Cards[e.card].Abilities[e.ab].Prog
}

// runProg executes from res.pc until the program ends or an op suspends.
func (s *State) runProg(k *prep) {
	prog := s.prog(k)
	for int(s.res.pc) < len(prog) {
		in := &prog[s.res.pc]
		if !s.exec(k, in) {
			s.ctl = ctlResolve
			return
		}
		if s.over {
			return
		}
		if k.gen != nil && s.parkReturnAsk() {
			return
		}
		if k.gen != nil && s.g.nc.n != 0 {
			s.res.ask, s.res.askP = raGenReplOrder, s.g.ro.p // a damage event parked by the op (parkNC)
			s.ctl = ctlResolve
			return
		}
		if k.gen != nil && s.entryLatePark() {
			return // the order ask that follows a land's entry (repl_entryorder.go)
		}
		if s.res.ask == raGenCmdZone {
			s.ctl = ctlResolve // a commander's CR 903.9b move was parked at a hand/library destination
			return
		}
		if s.g.nask != 0 && s.midResAsk(k) {
			return
		}
		if s.res.ask == raGenEnterDefender {
			s.ctl = ctlResolve // the entry's CR 508.4 defender ask (zone_changezoneparams.go)
			return
		}
		if len(s.cold.parked) != 0 && s.parkedWaiting() {
			s.ctl = ctlExileOrder
			return
		}
		s.res.pc++
		s.loopBack(k)
	}
	s.finishResolve(k, false)
}

// loopBack handles OpRepeat bodies: when pc passes the end of a loop body
// it jumps back while iterations remain.
func (s *State) loopBack(k *prep) {
	if k.gen != nil {
		s.loopBackGen()
	}
	r := &s.res
	for r.loopD > 0 {
		d := r.loopD - 1
		if r.pc != r.loopPC[d] {
			return
		}
		// loopPC is the first pc after the body; loopN the remaining runs;
		// an OpRepeatWhile loop (its header is the instruction before the
		// body) also ends when its condition fails after a pass
		if in := &s.prog(k)[r.loopS[d]-1]; in.Op == kop.OpRepeatWhile && !s.condEval(k, kop.CondID(in.C), s.ctxEntry(&r.e)) {
			r.loopD--
			continue
		}
		if r.loopN[d] > 1 {
			r.loopN[d]--
			r.pc = r.loopS[d]
			return
		}
		r.loopD--
	}
}

// objRef resolves an object Ref to a battlefield slot (NoSlot if it is not
// on the battlefield or no longer the same object).
func (s *State) objRef(k *prep, r kop.Ref) Slot {
	o := s.objRefObj(r)
	if o == NoObj {
		return NoSlot
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return NoSlot
	}
	if r == kop.RefSelf && ob.ver != s.res.e.srcVer && s.res.e.kind != seSpell {
		return NoSlot
	}
	if r == kop.RefTriggerObj && s.res.trigGone {
		return NoSlot // the triggering permanent left and came back meanwhile: a new object (CR 400.7)
	}
	return ob.slot
}

func (s *State) objRefObj(r kop.Ref) Obj {
	if r >= kop.RefChosen && s.k().gen != nil {
		if o, ok := s.objRefGen(r); ok {
			return o
		}
	}
	e := &s.res.e
	switch r {
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		t := s.tgtCode(sl)
		if t == NoTarget || isPlayerOrNone(t) {
			return NoObj
		}
		return Obj(t)
	case kop.RefSelf:
		return e.obj
	case kop.RefTriggerObj:
		return e.trigObj
	case kop.RefLastCreated:
		return s.res.lastTok
	case kop.RefChosen:
		return Obj(s.res.chosen)
	case kop.RefEach:
		return s.res.eachObj
	case kop.RefAttached:
		ob := &s.objs[e.obj]
		if ob.zone == kop.ZoneBattlefield {
			if a := s.perms[ob.slot].attached; a != NoSlot {
				return s.perms[a].obj
			}
			return NoObj
		}
		return s.lkiAttached(e.obj)
	}
	return NoObj
}

// playerRef resolves a player Ref (255 = none). RefEachPlayer and
// RefEachOpponent are expanded by the caller.
func (s *State) playerRef(k *prep, r kop.Ref) uint8 {
	e := &s.res.e
	switch r {
	case kop.RefController:
		return e.ctrl
	case kop.RefOpponent, kop.RefEachOpponent, kop.RefDefendingPlayer:
		return s.oneOpp(k, r)
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		t := s.tgtCode(sl)
		if isPlayerOrNone(t) && t != NoTarget {
			return CodeSeat(t)
		}
		if t != NoTarget {
			return s.ctrlOfObj(Obj(t))
		}
		return 255
	case kop.RefTriggerPlayer:
		return e.trigPl
	case kop.RefTriggerTgtCtrl:
		// gorge TriggeredTargetController: the controller of the trigger's
		// target object, read live (a referent that left the battlefield
		// since it was dealt the damage reads its owner). No referent (a
		// trigger kind that binds no object) keeps the queue-time player.
		if o := e.trigObj; o != NoObj {
			return s.ctrlOfObj(o)
		}
		return e.trigPl
	case kop.RefTargetController:
		return s.tgtCtrlSlot(k, 0)
	case kop.RefTgtCtrl1, kop.RefTgtCtrl2:
		return s.tgtCtrlSlot(k, int(r-kop.RefTgtCtrl1)+1) // tgt_stackability.go
	case kop.RefTargetOwner:
		t := s.tgtCode(0)
		if t == NoTarget || isPlayerOrNone(t) {
			return 255
		}
		return s.objs[t].owner()
	case kop.RefSelf:
		return e.ctrl
	case kop.RefOppNonRemCtrl:
		return s.oppsNonRemCtrl().at(0) // filter_defined.go: the first of the set
	case kop.RefSrcOwner:
		return s.srcOwner()
	case kop.RefPromised:
		return s.promisedSeat(k, e) // kw_gift.go
	case kop.RefAttachedCtrl:
		return s.attachedCtrl(s.objRefObj(kop.RefAttached))
	}
	if k.gen != nil {
		return s.refPlayer(k, r, s.ctxEntry(&s.res.e))
	}
	return 255
}

// srcCtrl is the controller of the resolving entry's source as it deals damage:
// the permanent's current controller while it is still the same object on the
// battlefield, else the last-known controller from the departed battlefield
// record (Stiltzkin gave the source away with its ability on the stack, then it
// left; CR 702.15b gains the life for the source's last-known controller). Only
// an entry with no such record falls back to the ability's controller.
func (s *State) srcCtrl(k *prep) uint8 {
	if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
		return s.perms[sl].ctrl
	}
	e := &s.res.e
	if e.kind != seSpell {
		if sl := s.departedSrcSlot(e); sl != NoSlot {
			return s.perms[sl].ctrl
		}
	}
	return e.ctrl
}

func (s *State) ctrlOfObj(o Obj) uint8 {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return s.perms[ob.slot].ctrl
	}
	return ob.owner()
}

// value evaluates a ValueRef.
// enterCountValue is value for the amount of an enters-with-counters
// replacement, which counts for the entering permanent (sl, controlled by
// ctrl): its "other" is that permanent, not the source of the effect that
// put it onto the battlefield (Rescue Skiff's trigger returning Luxknight
// Breacher, which counts the Skiff), and its "you" is that permanent's
// controller, not the effect's (All-Fates Stalker's return of Gigastorm Titan
// under Bioengineered Future counts lands that entered under the Titan's
// controller).
func (s *State) enterCountValue(k *prep, v kop.ValueRef, sl Slot, ctrl uint8) int {
	if v.K == kop.VExpr {
		c := s.ctxEntry(&s.res.e)
		c.ctrl = ctrl
		c.src, c.srcSl = s.perms[sl].obj, sl
		return s.expr(k, v.N, c)
	}
	if v.K != kop.VCountFilter {
		return s.value(k, v)
	}
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		if s.matchPerm(k, kop.FilterID(v.N), m.first(), ctrl, sl) {
			n++
		}
	}
	return n
}

func (s *State) value(k *prep, v kop.ValueRef) int {
	e := &s.res.e
	switch v.K {
	case kop.VConst:
		return int(v.N)
	case kop.VX:
		return int(e.x)
	case kop.VNegX:
		return -int(e.x)
	case kop.VLastDamage:
		return int(s.res.lastDmg)
	case kop.VTriggerAmount:
		return int(e.trigAmt)
	case kop.VCardsInHand:
		return int(s.pl[e.ctrl].handN)
	case kop.VLifeGainedTurn:
		return int(s.pl[e.ctrl].lifeGained)
	case kop.VSelfPower, kop.VSelfToughness:
		sl := s.objRef(k, kop.RefSelf)
		if sl == NoSlot {
			c := &k.cards[e.card]
			pw, t := int(c.power), int(c.tough)
			if e.kind != seSpell && e.obj != NoObj {
				// an ability whose source has left: its last known P/T
				// (CR 608.2h; Threefold Thunderhulk sacrificed with its
				// "Gnomes equal to its power" trigger waiting)
				pw, t = s.selfPTLKI(k, e, pw, t)
			}
			if v.K == kop.VSelfPower {
				return pw
			}
			return t
		}
		p, t := s.pt(k, sl)
		if v.K == kop.VSelfPower {
			return p
		}
		return t
	case kop.VTargetPower, kop.VTargetToughness:
		sl := s.objRef(k, kop.RefTarget0)
		if sl == NoSlot {
			if l := s.lk(s.res.e.li); k.gen != nil && s.res.e.li != 0 && s.res.e.kind == seSpell {
				if v.K == kop.VTargetPower {
					return int(l.power)
				}
				return int(l.tough)
			}
			return 0
		}
		p, t := s.pt(k, sl)
		if v.K == kop.VTargetPower {
			return p
		}
		return t
	case kop.VCountFilter:
		n := 0
		src := s.objRef(k, kop.RefSelf)
		for m := s.occ; m.any(); m = m.rest() {
			if s.matchPerm(k, kop.FilterID(v.N), m.first(), e.ctrl, src) {
				n++
			}
		}
		return n
	case kop.VCountersSelf:
		sl := s.objRef(k, kop.RefSelf)
		if sl == NoSlot {
			if dsl := s.selfCountersLKI(&s.res.e, s.res.e.obj); dsl != NoSlot {
				return s.perms[dsl].ctrN(s, int(v.N)) // a departed source's last counters (CR 608.2h)
			}
			if n, ok := s.selfDepartCtr(&s.res.e, s.res.e.obj, kop.CounterKind(v.N)); ok {
				return n // its slot was reused: the departure snapshot's (ctr_selfdepart.go)
			}
			if k.gen != nil && s.res.e.li != 0 && s.res.e.kind == seAbility {
				return s.lkiCounter(s.res.e.li, kop.CounterKind(v.N)) // a sacrificed source's last counters (Ravenous Amulet)
			}
			return 0
		}
		return s.perms[sl].ctrN(s, int(v.N))
	case kop.VCountersTarget:
		sl := s.objRef(k, kop.RefTarget0)
		if sl == NoSlot {
			return 0
		}
		return s.perms[sl].ctrN(s, int(v.N))
	case kop.VExpr:
		return s.expr(k, v.N, s.ctxEntry(e))
	}
	panic("kernel: value kind not implemented")
}

// srcKw is the resolving source's keywords (battlefield if still there,
// else its card's printed keywords).
func (s *State) srcKw(k *prep) kop.KwMask {
	if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
		return s.kw(k, sl)
	}
	kw := k.cards[s.res.e.card].kw
	if e := &s.res.e; e.kind != seSpell {
		kw = s.srcLeftKw(kw, e) // kw_srcleft.go: a source that left uses its last known keywords (CR 608.2h)
	}
	if k.gen != nil && k.gen.anyStackLife && s.res.e.kind == seSpell {
		kw |= s.stackLifelinkKw(k) // static_stacklifelink.go
	}
	if s.res.e.kind == seSpell {
		if s.res.e.remVer&spellGainedDeathtouch != 0 {
			kw |= kop.KwDeathtouch
		}
		if s.res.e.remVer&spellGainedLifelink != 0 {
			kw |= kop.KwLifelink
		}
	}
	return kw
}

// grantSpellKw gives the spell object o is on the stack as the keywords of kw an
// entry can carry; its damage reads them as it resolves (srcKw; gorge
// effects/damage.go HasKeyword on the stack object).
func (s *State) grantSpellKw(o Obj, kw kop.KwMask) {
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			if kw&kop.KwDeathtouch != 0 {
				e.remVer |= spellGainedDeathtouch
			}
			if kw&kop.KwLifelink != 0 {
				e.remVer |= spellGainedLifelink
			}
			return
		}
	}
}

// damage deals n damage from the resolving source to an object or player
// target code and returns the amount dealt after prevention and replacement
// (0: none; CR 120.1).
func (s *State) damageTo(k *prep, code uint8, n int) int {
	if n <= 0 {
		return 0
	}
	e := &s.res.e
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbProtAll|rbProtCol|rbPlusDmg|rbShield|rbPrevCol|rbPrevMill|rbSelfDmg) != 0 {
		if s.parkNC(k, s.srcCtrl(k), s.objRef(k, kop.RefSelf), e.card, code, n, s.srcKw(k)) {
			return n // gorge's replacement-order ask, after the resolution
		}
		if n = s.replDamage(k, s.srcCtrl(k), s.objRef(k, kop.RefSelf), e.card, code, false, n); n <= 0 {
			return 0
		}
	}
	kw := s.srcKw(k)
	if k.gen != nil && k.gen.anyInfect && (s.srcInfect(k) || isObjTarget(code) && s.srcWither(k, kw)) {
		s.infectDamageTo(k, code, n, kw) // wither is infect's creature half (a player takes ordinary damage)
		return n
	}
	if isPlayerOrNone(code) {
		s.loseLife(k, CodeSeat(code), n)
	} else {
		ob := &s.objs[code]
		if ob.zone != kop.ZoneBattlefield {
			return 0
		}
		sl := ob.slot
		s.touch(Obj(code))
		p := &s.perms[sl]
		s.dmgPerm(k, sl, p, n)
		if kw&kop.KwDeathtouch != 0 {
			p.markDT(1) // a planeswalker too (gorge effects/damage.go, combat.go: any damaged permanent)
		}
		if k.gen != nil && k.gen.dmgBy != nil {
			s.noteDmgBy(k, s.objRef(k, kop.RefSelf), sl) // DamagedBy marks (filter_combatwords.go)
		}
	}
	s.res.lastDmg = int16(n)
	if kw&kop.KwLifelink != 0 {
		s.gainLife(k, s.srcCtrl(k), n)
	}
	if k.gen != nil && k.gen.anyPermMark {
		s.markDealt(k, s.objRef(k, kop.RefSelf), false) // filter_permmarks.go
	}
	if s.subs[kop.TrigDamageDone].any() || k.gen != nil && (s.subs[kop.TrigDamageDealtOnce].any() || k.gen.graveDmg) {
		if sl := s.objRef(k, kop.RefSelf); sl != NoSlot {
			s.fireDamage(k, sl, int16(n), isPlayerOrNone(code), false, code)
		} else if k.gen != nil && e.obj != NoObj {
			// a spell's damage, or an ability's whose source is gone
			// (Niv-Mizzet: the sacrificed Fanatical Firebrand's, pair 3334)
			s.fireDamageSrc(k, e.obj, e.ctrl, int16(n), dmgPl(code), code, nil)
		}
	}
	return n
}

// refCode turns an object/player Ref into a damage target code.
func (s *State) refCode(k *prep, r kop.Ref) uint8 {
	switch r {
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2, kop.RefTarget3, kop.RefTarget4:
		sl, _ := tgtSlot(r)
		return s.tgtCode(sl)
	case kop.RefController, kop.RefOpponent, kop.RefTriggerPlayer, kop.RefDefendingPlayer, kop.RefTargetController, kop.RefEachOpponent, kop.RefPromised:
		p := s.playerRef(k, r)
		if p == 255 {
			return NoTarget
		}
		return PlayerCode(p)
	}
	o := s.objRefObj(r)
	if o == NoObj {
		return NoTarget
	}
	return uint8(o)
}

// forPlayers runs f for the player(s) a Ref names.
func (s *State) players(k *prep, r kop.Ref) seatSeq {
	switch r {
	case kop.RefEachPlayer:
		// gorge's "each player" set is AliveFrom(controller): the resolving
		// controller first, then the other seats in turn order (on the
		// controller's own turn that is APNAP).
		return s.aliveFrom(s.res.e.ctrl)
	case kop.RefEachOpponent:
		// gorge Defined$ Opponent: AliveFrom(controller) without it
		return s.oppsFrom(s.res.e.ctrl)
	}
	return seatOne(s.playerRef(k, r))
}

// oneOpp is a single-player read of an opponent ref. At two seats it is the
// one opponent. At more, the defending player is the player the resolving
// source is attacking (gorge binds c.DefendingPlayer from the attacking
// damage or attack source), and an opponent ref read as one player is the
// first of gorge's Defined$ Opponent list, AliveFrom(controller) without the
// controller: the next seat still in the game (twoPilesPlayer and the other
// first-player reads).
func (s *State) oneOpp(k *prep, r kop.Ref) uint8 {
	e := &s.res.e
	if !s.res.active {
		return s.oneOppOf(k, r, nil, e.ctrl, e.obj)
	}
	return s.oneOppOf(k, r, e, e.ctrl, e.obj)
}

// oneOppOf is oneOpp for entry e (nil: none), controller ctrl and source
// obj. Past two seats an attack trigger's defending player is the one bound
// as it was queued (defSeat), else the seat its attacking source attacks.
func (s *State) oneOppOf(k *prep, r kop.Ref, e *stackEntry, ctrl uint8, obj Obj) uint8 {
	if MaxSeats == 2 {
		return NextSeat(ctrl)
	}
	if r == kop.RefDefendingPlayer && e != nil && e.kind == seTrigger && e.defSeat != 0 {
		return e.defSeat - 1
	}
	if r == kop.RefDefendingPlayer && obj != NoObj {
		if ob := &s.objs[obj]; ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].flags&pfAttacking != 0 {
			if p := s.atkSeat(k, ob.slot); p != NoTarget {
				return p
			}
		}
	}
	return s.nextAlive(ctrl)
}

// tgtCode is the resolving entry's target i, or NoTarget when it was
// chosen but is illegal now.
func (s *State) tgtCode(i int) uint8 {
	if s.res.bad&(1<<i) != 0 {
		return NoTarget
	}
	return s.res.e.tgt[i]
}
