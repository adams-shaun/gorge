package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A sacrifice-only mana source (an Eldrazi Spawn token's "Sacrifice this
// creature: Add {C}", CR 605.1a): the ability's cost has no {T}, so a tapped
// source can still pay, and summoning sickness never bars it. gorge prices
// it as a last-resort alternative of its source's payment-window ability
// (pay/plancost.go PlanLastResortCostOK, payment_plan.go
// paymentPlanManaUnitsOnlyCompute), so the planner uses it only when no
// plan of normal sources pays. The census extends it tapped or untapped
// alike, as does the manual ward / unless window (mana_window.go windowTap):
// gorge's availableManaAbilities gates only tap costs.

// DebugIgnoreSpawnMana makes prepare drop sacrifice-only mana abilities.
// The spawnmana lockstep's mutation test flips it to prove the pool
// exercises them (the reference keeps them, so a play that used one
// diverges).
var DebugIgnoreSpawnMana bool

// anySacSource reports whether p controls a source a payment plan could
// sacrifice: an untapped tap-and-sacrifice source (a Treasure) or any
// sacrifice-only one (a Spawn, tapped or not). It is the cheap gate before
// kickWitness copies the state to count the sacrifice triggers.
func (s *State) anySacSource(k *prep, p uint8) bool {
	if !k.sacMana {
		return false
	}
	for b := s.ctrlM[p]; b.any(); b = b.rest() {
		sl := b.first()
		pc := s.manaPCAt(k, sl)
		if pc.manaSac && s.canTapForManaPC(k, sl, pc) || pc.manaSacOnly && s.canSacOnlyNow(k, sl, pc) {
			return true
		}
	}
	return false
}

// witnessWaits is how many stacked triggers gorge's witness execution lets resolve (one
// pass each) before it gives up (potential_script.go witnessReachesFrom, waits >= 4).
const witnessWaits = 4

// xWitnessEats: gorge does not take the witness for spell c's {X} at x, so it plans the
// largest X that keeps its sources (potentialWitnessKeeping). It is not taken when the
// witness sacrifices the last permanent the spell could target (For the Common Good paid
// with the Treasure it would copy), and when its sacrifices stack more triggers than the
// wait budget lets resolve before the play is offered again (kickWitness; Nyxborn Hydra's
// X paid with three Eldrazi Spawn beside two Writhing Chrysalis).
func (s *State) xWitnessEats(k *prep, p uint8, c CardID, cost *kop.ManaCost, x int, excl Obj) bool {
	bestowed := s.buildLive() && s.bld().e.bestowedEntry() // its one target is the enchant creature
	targeted := bestowed || tspec(k, c, spellAb(k, c), 0) != nil
	stacked := s.subs[kop.TrigSacrificed].any()
	if !s.anySacSource(k, p) || !targeted && !stacked {
		return false
	}
	t := s.fork()
	defer s.unfork()
	n0 := t.npend
	var pk planner
	pk.try = true
	t.tapForPK(k, p, cost, x, excl, false, false, &pk)
	if !pk.found {
		return false
	}
	if bestowed {
		var buf [MaxOptions]uint8
		if t.targetCands(k, &k.t.Cards[c].BestowTgt, p, NoSlot, t.bld().e.obj, &buf) == 0 {
			return true
		}
	} else if targeted && !t.spellTargetsFeasible(k, c, p) {
		return true
	}
	return stacked && int(t.npend)-int(n0) > witnessWaits
}

// sacSourcesHeld takes p's sacrificing mana sources out of its control mask for a witness
// that keeps them, and returns them to put back.
func (s *State) sacSourcesHeld(k *prep, p uint8) slotMask {
	var held slotMask
	for b := s.ctrlM[p]; b.any(); b = b.rest() {
		if pc := s.manaPCAt(k, b.first()); pc.manaSac || pc.manaSacOnly {
			held = held.or(slotBit(b.first()))
		}
	}
	s.ctrlM[p] = s.ctrlM[p].andNot(held)
	return held
}

// xWitnessOffered: gorge has a witness for p's {X} cast of hand card o, at the largest X or
// at a smaller one that keeps the sacrificing sources.
func (s *State) xWitnessOffered(k *prep, p uint8, c CardID, o Obj) bool {
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	var av avail
	s.availFor(k, p, &av)
	if !s.xWitnessEats(k, p, c, &cost, max(0, av.maxX(&cost, pipMaskOf(&cost))), o) {
		return true
	}
	held := s.sacSourcesHeld(k, p)
	s.availFor(k, p, &av)
	s.ctrlM[p] = s.ctrlM[p].or(held)
	return av.affords(&cost, pipMaskOf(&cost), 0)
}

// kickWitness mirrors gorge's potentialWitnessReaches for a kicked cast
// whose plan sacrifices a source: the witness lets at most four stacked
// triggers resolve before it gives up (potential_script.go's wait budget),
// so a plan whose sacrifices trigger more than four abilities is not
// offered. The plan is executed on a copy of the state.
func (s *State) kickWitness(k *prep, p uint8, cost *kop.ManaCost) bool {
	if !s.subs[kop.TrigSacrificed].any() {
		return true
	}
	if !s.anySacSource(k, p) {
		return true // no source the plan could sacrifice
	}
	tmp := s.fork()
	defer s.unfork()
	n0 := tmp.npend
	var pk planner
	pk.try = true
	tmp.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	if !pk.found {
		return true
	}
	return int(tmp.npend)-int(n0) <= witnessWaits
}

// canSacOnlyForMana reports whether the autopay planner may price sl as a
// last-resort sacrifice-only source: not stripped of its abilities. Its cost
// has no {T}, so a tapped source pays as well as an untapped one (CR 602.2b).
func (s *State) canSacOnlyForMana(k *prep, sl Slot, pc *pcard) bool {
	if !k.sacOnlyMana || !pc.manaSacOnly {
		return false
	}
	return s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
}

// canSacOnlyAt is canSacOnlyForMana for a caller that has only the slot.
func (s *State) canSacOnlyAt(k *prep, sl Slot) bool {
	return s.canSacOnlyForMana(k, sl, s.manaPCAt(k, sl))
}

// canSacOnlyNow is the mana window's membership test: a sacrifice-only
// ability needs no untap, only the ability itself (manaPCGen already
// returns noManaPC for a source that lost its abilities).
func (s *State) canSacOnlyNow(k *prep, sl Slot, pc *pcard) bool {
	return k.sacOnlyMana && pc.manaSacOnly && s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
}

// canSacOnlyNowAt is canSacOnlyNow for a caller that has only the slot.
func (s *State) canSacOnlyNowAt(k *prep, sl Slot) bool {
	return s.canSacOnlyNow(k, sl, s.manaPCAt(k, sl))
}

// availTappedSac adds a tapped sacrifice-only source's mana to a (the
// manual ward / unless window's availability; availFor is the planner's
// census, which prices only untapped sources).
func (s *State) availTappedSac(k *prep, p uint8, a *avail) {
	if !k.sacOnlyMana {
		return
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped == 0 {
			continue
		}
		pc := s.manaPCAt(k, sl)
		if s.canSacOnlyNow(k, sl, pc) {
			a.addUnits(s.srcMaskAt(pc, sl), pc.manaN)
		}
	}
}

// resolveLowerCounterTriggers resolves the pending triggers a lowered
// cast's payment created before the cast's own triggers are placed. gorge
// lowers a cast by activating each witness source at priority
// (payexec.Execution), so a sacrifice's trigger goes on the stack before the
// spell does, and the lowering passes priority until it resolves. A trigger
// that asks nothing when it is put on the stack or when it resolves therefore
// lands its whole effect at once: a program with no target, no mode and no
// "may", that runs through without suspending (runsPreCast, tried on a copy).
// One that asks is answered by the lowering's default (botpolicy.Clamp on an
// empty intent: the fewest, first options), and stays pending here for the
// ordinary stack path; so do a program that adds mana (the lowering's pool
// check would end the witness) and an optional trigger. A trigger that
// mills, loses life, gains life or draws for a target player is answered
// with the first player gorge's default answer names.
func (s *State) resolveLowerCounterTriggers(k *prep, n0 uint8) {
	b := s.bld()
	instant := b.e.kind == seSpell && (k.cards[b.e.card].instant || s.castFlash(k, b.e.ctrl, b.e.card))
	for i := int(n0); i < int(s.npend); {
		pt := &s.pend[i]
		if int(pt.card) >= len(k.t.Cards) {
			i++
			continue
		}
		cd := &k.t.Cards[pt.card]
		if int(pt.ab) >= len(cd.Abilities) {
			i++
			continue
		}
		a := &cd.Abilities[pt.ab]
		var op kop.Op
		if len(a.Prog) == 1 {
			op = a.Prog[0].Op
		}
		// a one-op program with no ask: the counter, the mill and the life or draw of a player
		// (a target player gorge's default answer names)
		plTgt := a.NTargets == 1 && a.Targets[0].Kind == kop.TgtPlayer
		mills := op == kop.OpMill && plTgt && a.Prog[0].C&kop.MvMayMill == 0
		lifeDraw := (op == kop.OpLoseLife || op == kop.OpGainLife || op == kop.OpDraw) && (a.NTargets == 0 || plTgt)
		single := len(a.Prog) == 1 && (op == kop.OpPutCounter && a.NTargets == 0 || mills || lifeDraw)
		if a.Kind != kop.AbTriggered || a.NModes != 0 || a.Trig.Flags&kop.TfOptional != 0 {
			i++
			continue
		}
		e := stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl,
			trigObj: pt.trigObj, trigPl: pt.trigPl, trigAmt: pt.trigAmt, tgt: noTargets}
		if instant {
			// the lowering is done as soon as the play is offered, and an
			// instant-speed cast is offered with the trigger still on the stack
			// (payexec.priority): the trigger goes on below the spell
			if pt.ab >= abCascade || a.NTargets != 0 && !(single && plTgt) {
				i++
				continue
			}
			if plTgt {
				e.tgt[0], e.nt = PlayerTarget0, 1
				if !s.targetOK(k, &a.Targets[0], pt.ctrl, NoSlot, pt.obj, PlayerTarget0) {
					e.tgt[0] = PlayerTarget0 + 1
				}
			}
			s.pushBelowCast(k, e, pt)
			copy(s.pend[i:], s.pend[i+1:s.npend])
			s.npend--
			continue
		}
		if !single && (a.NTargets != 0 || !s.runsPreCast(k, e, a.Prog)) {
			i++
			continue
		}
		save := s.res
		s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		switch {
		case single && op == kop.OpPutCounter:
			s.putCounterProg(k, &a.Prog[0])
		case single:
			if plTgt {
				s.res.e.tgt[0] = PlayerTarget0
				if !s.targetOK(k, &a.Targets[0], pt.ctrl, NoSlot, pt.obj, PlayerTarget0) {
					s.res.e.tgt[0] = PlayerTarget0 + 1
				}
			}
			s.exec(k, &a.Prog[0])
		default:
			s.runPreCast(k, a.Prog)
		}
		s.res = save
		copy(s.pend[i:], s.pend[i+1:s.npend])
		s.npend--
	}
}

// pushBelowCast puts trigger entry e on the stack ahead of the cast being built,
// whose own push (CR 601.2a) comes after the lowering: what takePending does for a
// trigger that asks nothing, without the build it would otherwise take over.
func (s *State) pushBelowCast(k *prep, e stackEntry, pt *pendingTrig) {
	saved := *s.bld()
	*s.bld() = build{e: e, stage: stageFirst[seTrigger]}
	if k.gift {
		s.giftPushed(k, pt)
	}
	s.pushBuild(k)
	s.syncBuild()
	*s.bld() = saved
}

// runsPreCast reports whether trigger entry e's program runs to its end
// without suspending on an ask, on a copy of the state, and adds no mana.
func (s *State) runsPreCast(k *prep, e stackEntry, prog []kop.Instr) bool {
	if len(prog) == 0 {
		return false
	}
	for i := range prog {
		if prog[i].Op == kop.OpAddMana {
			return false
		}
	}
	t := s.fork()
	defer s.unfork()
	t.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return t.runPreCast(k, prog)
}

// runPreCast executes prog from the top outside a resolution (runProg
// without the stack entry's cleanup) and reports whether it ran through; false
// when an op asked, or parked an order or a replacement choice.
func (s *State) runPreCast(k *prep, prog []kop.Instr) bool {
	for int(s.res.pc) < len(prog) {
		if !s.exec(k, &prog[s.res.pc]) {
			return false
		}
		if s.over {
			return true
		}
		if k.gen != nil && (s.g.pre.state == preReturn || s.g.nc.n != 0 || s.entryLatePark()) {
			return false
		}
		if s.g.nask != 0 && s.midResAsk(k) || len(s.cold.parked) != 0 && s.parkedWaiting() {
			return false
		}
		s.res.pc++
		s.loopBack(k)
	}
	return true
}

// putCounterProg is the OpPutCounter effect of the interpreter (interp.go),
// run outside a resolution for a pre-cast trigger.
func (s *State) putCounterProg(k *prep, in *kop.Instr) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	n := s.placeCounters(k, sl, int(in.C), s.value(k, in.A), s.perms[sl].ctrl, ctrPutEffect)
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(in.C))
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.g.lastCK = kop.CounterKind(in.C) // CounterAddedOnce reads the kind
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(n))
	}
}

// flushPrefixTriggers puts the triggers a scripted prefix just queued onto
// the stack before the play itself is applied. gorge answers each prefix
// activation as its own decision, so its next priority round drains the
// trigger queue before the play is submitted; a sacrifice made while paying
// the play therefore sits below the play's own ability (seed 640: Writhing
// Chrysalis below Drownyard Lurker's activation, whose discard trigger is
// queued later and joins no order choice). A trigger with a target or a mode
// is answered with the script's first option; an order choice (two triggers at
// once) is left pending for the ordinary priority path.
func (s *State) flushPrefixTriggers(k *prep) {
	for s.npend != 0 {
		p := s.active
		n := s.pendCount(p)
		if n == 0 {
			p = 1 - p
			n = s.pendCount(p)
		}
		if n != 1 {
			return // two or more: gorge asks for an order first
		}
		i := -1
		for j := 0; j < int(s.npend); j++ {
			if s.pend[j].ctrl == p {
				i = j
				break
			}
		}
		if i < 0 {
			return
		}
		decides := s.pendNeedsDecision(k, i)
		s.takePending(k, i)
		if decides {
			// the script's answer to the trigger's ask is the first option (the search
			// branches on the answers in order, and a target does not change what the
			// prefix pays), given before the play is submitted
			for s.ctl == ctlBuild && !s.over {
				var d Decision
				s.StepOne(&d)
				if d.N == 0 || s.ctl != ctlBuild {
					break
				}
				s.Apply(d.Options[0])
			}
		}
	}
}

// pendNeedsDecision reports whether pending trigger i would pose a choice
// (targets or modes) when built. A delayed trigger (ab >= abDelayed) carries
// its own body and no printed target list, so it is placed directly.
func (s *State) pendNeedsDecision(k *prep, i int) bool {
	pt := &s.pend[i]
	if pt.ab >= abCascade || int(pt.card) >= len(k.t.Cards) {
		return false
	}
	cd := &k.t.Cards[pt.card]
	if int(pt.ab) >= len(cd.Abilities) {
		return false
	}
	a := &cd.Abilities[pt.ab]
	return a.NTargets != 0 || a.NModes != 0
}
