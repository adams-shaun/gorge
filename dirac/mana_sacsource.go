package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

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
	anySac := s.anySacSource(k, p)
	diesTrig := s.subs[kop.TrigDies].or(s.subs[kop.TrigLeaves]).any()
	if !anySac || !targeted && !stacked && !diesTrig {
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
	} else if targeted {
		// the X-bound target is judged at the X the fork's mana still pays
		if !t.spellTargetsFeasible(k, c, p) || !t.spellXBoundPayable(k, c, p, cost) {
			return true
		}
	}
	// the wait budget counts the triggers the witness stacks, before any resolve
	if stacked && int(t.npend)-int(n0) > witnessWaits {
		return true
	}
	if plannerConsumesSac(&pk) {
		// gorge's potentialWitnessReaches runs the witness's activations and
		// requires the play still offered: a sacrifice whose trigger resolves
		// to end the game before the cast (Al Bhed Salvagers draining the last
		// life) is no witness while one that keeps the source exists
		// (potentialWitnessKeeping). The kernel's pre-cast resolution is the
		// same walk.
		t.resolveLowerCounterTriggers(k, n0, k.cards[c].instant || t.castFlash(k, p, c))
		if t.over {
			return true
		}
	}
	return s.xWitnessEndsGame(k, p, t, n0, cost)
}

// xWitnessEndsGame: the witness fork t (taken from s before the cast's pre-cast triggers
// resolve) queued sacrifice triggers from n0 that, resolved as gorge's lowering lets them
// resolve, end the game, while a witness that keeps the sacrificing sources pays cost at
// X=0. gorge's potentialWitnessReaches (winOK=false) replays the witness on a clone, and
// witnessReachesMode leaves at once on c.G.Over, so the witness "does not reach" the play
// and potentialWitnessKeeping plans the largest X that keeps the sources (Choco-Comet's X
// beside the Treasure and Al Bhed Salvagers, p0 at 1 life). Only a live sorcery-speed
// build: an instant-speed cast is offered with the trigger still on the stack, below the
// spell, so the witness reaches it; with no keeping plan gorge takes the lethal one
// (potential_plan.go's winOK fallback; resolveLowerCounterTriggers ends the game).
func (s *State) xWitnessEndsGame(k *prep, p uint8, t *State, n0 uint8, cost *kop.ManaCost) bool {
	if t.npend <= n0 || !s.buildLive() {
		return false
	}
	b := s.bld()
	if b.e.kind != seSpell || k.cards[b.e.card].instant || s.castFlash(k, b.e.ctrl, b.e.card) {
		return false
	}
	// the guard above leaves only the sorcery-speed build, whose triggers the
	// lowering resolves before the cast (instant is false).
	t.resolveLowerCounterTriggers(k, n0, false)
	if !t.over {
		return false
	}
	var av avail
	held := s.sacSourcesHeld(k, p)
	s.availFor(k, p, &av)
	s.ctrlM[p] = s.ctrlM[p].or(held)
	return av.affords(cost, pipMaskOf(cost), 0)
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

// smallerXWitness is gorge's potentialWitnessSmallerX: the largest X below hi
// whose payment the offer walk still admits (its sacrifices leave a target and
// stack no more triggers than the wait budget lets resolve). ok is false when
// no smaller X pays.
func (s *State) smallerXWitness(k *prep, p uint8, c CardID, cost *kop.ManaCost, hi int, excl Obj) (int, bool) {
	stacked := s.subs[kop.TrigSacrificed].any()
	bestowed := s.buildLive() && s.bld().e.bestowedEntry()
	targeted := bestowed || tspec(k, c, spellAb(k, c), 0) != nil
	for x := hi - 1; x >= 0; x-- {
		t := s.fork()
		n0 := t.npend
		var pk planner
		pk.try = true
		t.tapForPK(k, p, cost, x, excl, false, false, &pk)
		found := pk.found
		ok := found && (!stacked || int(t.npend)-int(n0) <= witnessWaits)
		if ok && bestowed {
			var buf [MaxOptions]uint8
			ok = t.targetCands(k, &k.t.Cards[c].BestowTgt, p, NoSlot, t.bld().e.obj, &buf) != 0
		} else if ok && targeted {
			ok = t.spellTargetsFeasible(k, c, p) && t.spellXBoundPayable(k, c, p, cost)
		}
		s.unfork()
		if ok {
			return x, true
		}
	}
	return 0, false
}

// xWitnessOffered: gorge has a witness for p's {X} cast of hand card o, at the largest X or
// at a smaller one that keeps the sacrificing sources.
func (s *State) xWitnessOffered(k *prep, p uint8, c CardID, o Obj) bool {
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	var av avail
	s.availFor(k, p, &av)
	mx := max(0, av.maxX(&cost, pipMaskOf(&cost)))
	if !s.xWitnessEats(k, p, c, &cost, mx, o) {
		return true
	}
	// gorge's potentialWitnessSmallerX: the max X's payment consumes the last
	// permanent the spell could target (or stacks past the wait budget), so
	// gorge prices a smaller X whose payment keeps one (For the Common Good
	// paid with one of two Treasures; the other stays a target).
	if _, ok := s.smallerXWitness(k, p, c, &cost, mx, o); ok {
		return true
	}
	held := s.sacSourcesHeld(k, p)
	s.availFor(k, p, &av)
	s.ctrlM[p] = s.ctrlM[p].or(held)
	if s.g.mayPlayAny {
		// the keeping plan prices the printed pips first and the permission's
		// any-type rider when they have none (potentialModeCastPlan)
		rc := anyType(cost)
		return av.affords(&cost, pipMaskOf(&cost), 0) || av.affords(&rc, pipMaskOf(&rc), 0)
	}
	return av.affords(&cost, pipMaskOf(&cost), 0)
}

// lowerXEats taps the witness of an {X} cast whose largest-X payment xWitnessEats
// (lowerWitnessEx): gorge's potentialWitnessKeeping, the largest X that keeps
// every sacrificing source (priced under the cast's any-type rider when the
// printed pips have no such plan), else potentialWitnessSmallerX, the largest
// smaller X whose payment consumes them yet leaves a target. x is the largest X
// the sources pay.
func (s *State) lowerXEats(k *prep, p uint8, cost *kop.ManaCost, x int, excl Obj, anyColor bool) {
	held := s.sacSourcesHeld(k, p)
	var av avail
	s.availFor(k, p, &av)
	kc, kx := *cost, 0
	for pass := 0; pass < 2; pass++ {
		if pass == 1 {
			if s.bld().e.flags&sfAnyMana == 0 && !anyColor {
				break
			}
			kc = s.buildCost(k) // the rider's price (the witness priced the printed pips)
		}
		if anyColor {
			kx = av.maxXAnyColor(&kc)
		} else {
			kx = av.maxX(&kc, pipMaskOf(&kc))
		}
		if kx >= 0 {
			s.tapForRider(k, p, &kc, kx, excl, anyColor)
			s.ctrlM[p] = s.ctrlM[p].or(held)
			return
		}
	}
	s.ctrlM[p] = s.ctrlM[p].or(held)
	if sx, ok := s.smallerXWitness(k, p, s.bld().e.card, cost, x, excl); ok {
		x = sx
	}
	s.tapForRider(k, p, cost, x, excl, anyColor)
}

// xWitnessUnreachable: p's plain {X} cast of hand card o is paid by the printed
// sources (some X >= 0) yet xWitnessOffered refuses it, so gorge's verdict is
// "unsupported witness_unreachable", not "insufficient": the census is
// complete for it and no scripted prefix search runs (potentialPlans). For the
// Common Good with one Treasure beside a Plains and a {1} filter: the Treasure
// pays {G} at X = 0 and is the only token it could copy.
func (s *State) xWitnessUnreachable(k *prep, p uint8, o Obj) bool {
	c := s.objs[o].card
	if k.t.Cards[c].Cost.X == 0 || s.objs[o].zone != kop.ZoneHand {
		return false
	}
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	var av avail
	s.availFor(k, p, &av)
	return av.maxX(&cost, pipMaskOf(&cost)) >= 0 && !s.xWitnessOffered(k, p, c, o)
}

// kickWitness mirrors gorge's potentialWitnessReaches for a kicked cast
// whose plan sacrifices a source: the witness lowers the plan before the
// cast, so a spell whose targets the sacrifices consume is not offered
// (the kicked cast is no planned cast, targetPaidPlanned), and it lets at
// most four stacked triggers resolve before it gives up (potential_script.go's
// wait budget), so a plan whose sacrifices trigger more than four abilities
// is not offered. The plan is executed on a copy of the state, and a kicker that
// sacrifices is payable there or not offered.
func (s *State) kickWitness(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
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
	if ab := spellAb(k, c); ab != 255 && plannerConsumesSac(&pk) && !tmp.abilityTargetsFeasible(k, &k.t.Cards[c].Abilities[ab], p, NoSlot, 0xff) {
		return false
	}
	// the plan's mana abilities are activated before the kicker's cost is paid (CR 601.2g-h): a
	// Treasure the plan sacrificed is no longer there for a kicker that sacrifices an artifact
	if kc := &k.t.Cards[c].KickerCost; kc.Flags&kop.CostSac != 0 && plannerConsumesSac(&pk) && !tmp.pickCostPayable(k, p, kc, o) {
		return false
	}
	return !s.subs[kop.TrigSacrificed].any() || int(tmp.npend)-int(n0) <= witnessWaits
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
//
// The caller states whether the cast being lowered is instant-speed: the offer
// path walks this on a fork with no build live (bld holds the build only in
// ctlBuild), so reading its entry there would take the speed of whichever card
// was built or resolved last.
func (s *State) resolveLowerCounterTriggers(k *prep, n0 uint8, instant bool) {
	resolved := false
	for i := int(n0); i < int(s.npend); {
		if s.over {
			return
		}
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
		// a straight run of life losses, gains and draws, one of them aimed at a target
		// player gorge's default answer names (Al Bhed Salvagers: target opponent loses
		// 1 life and you gain 1)
		plRun := !single && plTgt && lifeRun(a.Prog)
		if a.Kind != kop.AbTriggered || a.NModes != 0 || a.Trig.Flags&kop.TfOptional != 0 {
			i++
			continue
		}
		e := stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl,
			trigObj: pt.trigObj, trigPl: pt.trigPl, defSeat: pt.defSeat, trigAmt: pt.trigAmt, tgt: noTargets}
		if instant {
			// the lowering is done as soon as the play is offered, and an
			// instant-speed cast is offered with the trigger still on the stack
			// (payexec.priority): the trigger goes on below the spell, a
			// player-targeted one aimed where gorge's default answer names (Al
			// Bhed Salvagers' lose-and-gain run as well as a one-op program)
			if pt.ab >= abCascade || a.NTargets != 0 && !(single && plTgt || plRun) {
				i++
				continue
			}
			if plTgt {
				e.tgt[0], e.nt = s.firstPlayerTgt(k, &a.Targets[0], pt), 1
			}
			s.pushBelowCast(k, e, pt)
			copy(s.pend[i:], s.pend[i+1:s.npend])
			s.npend--
			continue
		}
		if plRun {
			e.tgt[0], e.nt = s.firstPlayerTgt(k, &a.Targets[0], pt), 1
		}
		if !single && (a.NTargets != 0 && !plRun || !s.runsPreCast(k, e, a.Prog)) {
			i++
			continue
		}
		save := s.res
		s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		s.countPreCastResolve(k, &e)
		switch {
		case single && op == kop.OpPutCounter:
			s.putCounterProg(k, &a.Prog[0])
		case single:
			if plTgt {
				s.res.e.tgt[0] = s.firstPlayerTgt(k, &a.Targets[0], pt)
			}
			s.exec(k, &a.Prog[0])
		default:
			s.runPreCast(k, a.Prog)
		}
		s.res = save
		copy(s.pend[i:], s.pend[i+1:s.npend])
		s.npend--
		resolved = true
	}
	if resolved {
		// The pre-cast trigger resolved; the priority it returns to runs its
		// state-based check before the cast's build continues (CR 704.3). A
		// drain that took the last opponent -- a Treasure spent for a
		// lowered hybrid cast beside The Sackville-Bagginses -- ends the
		// game here (CR 704.5a), before the cast, exactly as gorge's
		// lowering visits the resulting priority, so the card stays in
		// hand. resolvePrefixStack takes the same pass after its
		// resolutions.
		s.sba(k)
	}
}

// lifeRun reports whether prog is only life losses, life gains and draws.
func lifeRun(prog []kop.Instr) bool {
	for i := range prog {
		switch prog[i].Op {
		case kop.OpLoseLife, kop.OpGainLife, kop.OpDraw:
		default:
			return false
		}
	}
	return len(prog) != 0
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
	t.countPreCastResolve(k, &e)
	return t.runPreCast(k, prog)
}

// countPreCastResolve tallies a trigger resolved ahead of the cast as the
// stack path's trigResolveOK does (countResolve): gorge resolves it off the
// stack like any other, so it counts toward Count$ResolvedThisTurn (Vito,
// Fanatic of Aclazotz's Treasure sacrificed paying for a spell is the
// second resolution after an earlier one, CR 603.2).
func (s *State) countPreCastResolve(k *prep, e *stackEntry) {
	if k.t.Cards[e.card].Abilities[e.ab].AFlags&kop.AfResolveCount != 0 {
		s.countResolve(k, e)
	}
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

// scriptStep poses the next decision of a scripted line in the order gorge's
// engine poses its options: scriptAnswers walks d.Options as built, and
// askTriggerOrder builds them in pending queue order (engine/rules/
// trigger_queue.go). stepRaw reproduces that pose order; StepOne's canonical
// sort is the lockstep's comparison order (types.go canonKey), not the
// engine's. Both the search's model (resolvePrefixStack) and the applied play
// (flushPrefixTriggers) answer from this one helper so they cannot drift.
func (s *State) scriptStep(d *Decision) { s.stepRaw(d) }

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
		for i := uint8(1); n == 0 && i < s.aliveCount(); i++ {
			p = s.nextAlive(p) // APNAP: the next player in turn order with a trigger
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
				s.scriptStep(&d)
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

// castNeedsEmptyStack reports whether the plain cast a is sorcery-speed: a
// card that is neither an instant nor flashed may be cast only with the stack
// empty (CR 307.1, 117.1a). gorge's script search keeps going through a pass
// while the prefix's trigger is on the stack until the play is offered
// (potential_script.go waitsForStack), so the trigger resolves first.
func (s *State) castNeedsEmptyStack(k *prep, a Action) bool {
	if a.Kind() != AkCast || a.Mode() != 0 {
		return false
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	return !k.cards[c].instant && !s.castFlash(k, s.prio, c)
}

// activateNeedsEmptyStack reports whether the activation a is sorcery-speed
// (SorcerySpeed$ True, which gorge's waitsForStack reads: equip, level up,
// outlast, a Class level, a land's sorcery-speed ability): it is offered only
// with the stack empty (CR 602.5d), as a cast is.
func (s *State) activateNeedsEmptyStack(k *prep, a Action) bool {
	if a.Kind() != AkActivate || k.gen == nil {
		return false
	}
	ob := &s.objs[Obj(a.Source())]
	c, ab := ob.card, a.Mode()
	switch {
	case ab&0x80 != 0:
		host := &s.objs[Obj(a.Target())]
		c, ab = host.card, ab&^0x80
		if host.zone == kop.ZoneBattlefield {
			c = s.grantCardAt(k, host.slot, ab)
		}
	case ob.zone == kop.ZoneBattlefield:
		c = s.cardAt(ob.slot)
	}
	if ab >= abCascade || int(ab) >= len(k.t.Cards[c].Abilities) {
		return false
	}
	return k.t.Cards[c].Abilities[ab].Cost.Flags&kop.CostSorcery != 0
}

// playNeedsEmptyStack is castNeedsEmptyStack or activateNeedsEmptyStack: the
// play a is offered only with the stack empty.
func (s *State) playNeedsEmptyStack(k *prep, a Action) bool {
	return s.castNeedsEmptyStack(k, a) || s.activateNeedsEmptyStack(k, a)
}

// resolvePrefixStack drives what a scripted prefix left behind before the
// play is applied: the triggers its activations queued (two or more wait for
// gorge KTriggerOrder, which blocks the payer priority, so the search's
// script answers it), the placements' dangling builds (takePending), and --
// for a sorcery-speed play, which is offered only with the stack empty --
// the resolutions, down to depth sp0, the other player passing as the search
// assumed. An optional trigger (Judge of Currents) asks its may-election as
// it resolves: the search answers the election's yes first (scriptAnswers
// lists Min == Max == 1's answers with the yes first, askTriggerOptional),
// so the kernel answers it here and lets the resolution finish. Any other
// ask is not the search's to answer: the play waits above it, as before.
func (s *State) resolvePrefixStack(k *prep, sp0 uint8, resolve bool) {
	resolved := false // the loop resolved a stack item: the SBA pass follows (CR 704.3)
	for !s.over {
		switch s.ctl {
		case ctlPriority:
			if s.npend > 0 {
				s.ctl = ctlPlaceTriggers // the queued triggers are placed first (CR 117.5)
				continue
			}
			if !resolve || s.sp <= sp0 {
				if resolve && resolved {
					// the resolutions are done: the priority they end at
					// runs its state-based check before the search's pass
					// is answered (CR 704.3). A line the script's
					// resolutions killed -- the equipped source's damage
					// trigger at the last opponent's 1 life -- ends the
					// search there, as gorge's visit does at the resulting
					// priority's c.G.Over, and the play is not offered.
					// Nothing resolved, nothing to check: the ordinary
					// path's call with the stack untouched skips it.
					s.sba(k)
				}
				return
			}
			spBefore := s.sp
			s.passes = 1
			s.pass(k)
			resolved = resolved || s.sp < spBefore
		case ctlPlaceTriggers:
			var d Decision
			s.scriptStep(&d) // the pose order scriptAnswers enumerates, shared with flushPrefixTriggers
			if s.ctl == ctlPlaceTriggers {
				// a posed ask here is the queued triggers' order (gorge
				// KTriggerOrder, one Min=Max=n multi-select whose only
				// scriptAnswers combination is the options in pose order,
				// the queue order): answer it as the script does, pick by
				// pick, so the triggers are placed before the play.
				// Anything else is the ordinary path's.
				if d.N == 0 || d.Kind != DkTriggerOrder {
					return
				}
				s.Apply(d.Options[0])
			}
		case ctlBuild:
			// the placement's takePending left the trigger's build
			// dangling: run it out (its asks, answered with the script's
			// first option, the pose order scriptAnswers enumerates)
			for s.ctl == ctlBuild && !s.over {
				var d Decision
				s.scriptStep(&d)
				if d.N == 0 || s.ctl != ctlBuild {
					break
				}
				s.Apply(d.Options[0])
			}
			if s.ctl == ctlBuild {
				return // stalled mid-build
			}
		case ctlResolve:
			if !resolve || s.res.ask != raOptional {
				return
			}
			var d Decision
			s.scriptStep(&d)
			if d.N == 0 || d.Sub != ChOptional {
				return
			}
			yes := -1
			for i := 0; i < int(d.N); i++ {
				if d.Options[i].Mode()&1 != 0 {
					yes = i
					break
				}
			}
			if yes < 0 {
				return
			}
			s.Apply(d.Options[yes])
			resolved = true
		default:
			return
		}
	}
}

// firstPlayerTgt is the player target a pre-cast trigger takes: the first
// seat in seat order its target spec admits, else the last seat (at two
// seats, seat 0 when legal, otherwise seat 1).
func (s *State) firstPlayerTgt(k *prep, ts *kop.TargetSpec, pt *pendingTrig) uint8 {
	q := uint8(0)
	for ; q+1 < s.nSeats(); q++ {
		if s.targetOK(k, ts, pt.ctrl, NoSlot, pt.obj, PlayerCode(q)) {
			break
		}
	}
	return PlayerCode(q)
}
