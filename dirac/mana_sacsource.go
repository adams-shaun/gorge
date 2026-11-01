package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A sacrifice-only mana source (an Eldrazi Spawn token's "Sacrifice this
// creature: Add {C}", CR 605.1a): the ability's cost has no {T}, so a tapped
// source can still pay, and summoning sickness never bars it. gorge prices
// it as a last-resort alternative of its source's payment-window ability
// (pay/plancost.go PlanLastResortCostOK, payment_plan.go
// paymentPlanManaUnitsOnlyCompute), so the planner uses it only when no
// plan of normal sources pays. Its census extends only an untapped source;
// the manual ward / unless window (mana_window.go windowTap) activates it
// tapped or untapped alike (gorge availableManaAbilities gates only tap
// costs).

// DebugIgnoreSpawnMana makes prepare drop sacrifice-only mana abilities.
// The spawnmana lockstep's mutation test flips it to prove the pool
// exercises them (the reference keeps them, so a play that used one
// diverges).
var DebugIgnoreSpawnMana bool

// availPayTapped is the availFor mode bit that lets the payment planner
// activate a tapped sacrifice-only source. gorge's potential census skips a
// tapped source (paymentPlanManaUnitsOnlyCompute), so the offer gate reads
// the untapped-only picture; a cast gorge reaches through a scripted prefix
// (potentialPrefixScript activates the source first) is paid with this bit
// set, exactly as the script's activations would leave it tapped.
const availPayTapped uint8 = 16

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
	return int(tmp.npend)-int(n0) <= 4
}

// canSacOnlyForMana reports whether the autopay planner may price sl as a
// last-resort sacrifice-only source: untapped (gorge's census extension
// skips a tapped source) and not stripped of its abilities.
func (s *State) canSacOnlyForMana(k *prep, sl Slot, pc *pcard) bool {
	if !k.sacOnlyMana || !pc.manaSacOnly {
		return false
	}
	p := &s.perms[sl]
	if p.flags&pfTapped != 0 && s.g.availX&availPayTapped == 0 {
		return false
	}
	return p.ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
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
// resolves a payment trigger before it puts the spell's cast triggers on the
// stack (its lowering pays before the cast), so a trigger whose whole
// program is one OpPutCounter lands its counter at once. Any other trigger
// stays pending for the ordinary stack path.
func (s *State) resolveLowerCounterTriggers(k *prep, n0 uint8) {
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
		if a.Kind != kop.AbTriggered || len(a.Prog) != 1 || a.Prog[0].Op != kop.OpPutCounter || a.NTargets != 0 || a.NModes != 0 || a.Trig.Flags&kop.TfOptional != 0 {
			i++
			continue
		}
		e := stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card, ab: pt.ab, ctrl: pt.ctrl,
			trigObj: pt.trigObj, trigPl: pt.trigPl, trigAmt: pt.trigAmt}
		save := s.res
		s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
		s.putCounterProg(k, &a.Prog[0])
		s.res = save
		copy(s.pend[i:], s.pend[i+1:s.npend])
		s.npend--
	}
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

// sacPrefixTriggers reports whether the scripted prefix for play a must
// activate a sacrifice-only gap whose sacrifice puts a trigger on the stack.
// gorge's prefix search (scriptSearch.visit) never passes priority, so at
// every node after such an activation the trigger still sits on the stack:
// a sorcery-speed cast is not offered there and the search cannot reach it.
// An instant, a flash cast or an activated ability is offered beside the
// trigger, so only a sorcery-speed cast is barred. Mixed gap kinds (an
// each-colour or restricted source beside the sacrifice-only ones) are not
// modelled: the prefix search's other branches are script_gen.go's.
func (s *State) sacPrefixTriggers(k *prep, a Action, p uint8) bool {
	sac := s.scriptSac(k, p)
	if !sac.any() || !isCastKind(a.Kind()) {
		return false
	}
	if s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).any() || k.gen.anyFilt && s.filtOnly(k, p) {
		return false
	}
	c := s.objs[a.Source()].card
	pc := &k.cards[c]
	if pc.instant || s.castFlash(k, p, c) {
		return false
	}
	if !s.sorcerySpeed(p) {
		return false
	}
	t := s.fork()
	defer s.unfork()
	var d Decision
	t.plainOptions(k, &d, 0)
	if hasOption(&d, a) {
		return false
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, sac, order[:], 0)
	n0 := t.npend
	for i := 0; i < n && !hasOption(&d, a); i++ {
		t.sacGap(k, p, order[i], a)
		if t.npend != n0 {
			return true // the trigger is on the stack: the cast waits for it
		}
		t.restrOptions(k, &d, p)
	}
	return t.npend != n0
}

// sacPrefixDeadEnd reports whether scriptPrefix's chain for play a
// sacrifices a tapped sacrifice-only gap and then runs out of gaps with a
// still unlisted. The sacrifice can take what the play needs: a Map token's
// "target creature you control" whose only creature is the tapped Eldrazi
// Spawn paying its {1} (CR 602.2b, 601.2c: no legal target left to
// announce). gorge's prefix search reaches the play only at a priority
// decision that lists it, and the oracle offers a scripted play only when
// the script completes on a clone (oracle gorge.go scriptCompletes, "script
// dead end"); offered, the kernel would begin the play after the prefix
// anyway (panic "spell/ability without a legal target", mzpauper
// Faeries vs Wildfire). The chain is scriptPrefix's own (gaps, restricted
// sources and tapped sacrifice-only sources in battlefield order),
// evaluated on a copy. A chain with no sacrifice changes no target and is
// scriptDeadEnd's.
func (s *State) sacPrefixDeadEnd(k *prep, a Action, p uint8, restr slotMask) bool {
	sac := s.scriptSac(k, p)
	if !sac.any() {
		return false
	}
	t := s.fork()
	defer s.unfork()
	var d Decision
	t.plainOptions(k, &d, 0)
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(p, t.scriptGaps(k, p).or(restr).or(sac), order[:], 0)
	var held [kop.NumManaKinds]uint8
	for i := 0; i < n && !hasOption(&d, a); i++ {
		if sac.has(order[i]) {
			t.sacGap(k, p, order[i], a)
		} else {
			t.tapGap(k, p, order[i], restr, &held)
		}
		t.restrOptions(k, &d, p)
	}
	return !hasOption(&d, a)
}

// flushPrefixTriggers puts the triggers a scripted prefix just queued onto
// the stack before the play itself is applied. gorge answers each prefix
// activation as its own decision, so its next priority round drains the
// trigger queue before the play is submitted; a sacrifice made while paying
// the play therefore sits below the play's own ability (seed 640: Writhing
// Chrysalis below Drownyard Lurker's activation, whose discard trigger is
// queued later and joins no order choice). Only a targetless trigger can be
// placed here without posing a decision; a target ask or an order choice is
// left pending for the ordinary priority path (the pool's prefix triggers
// have neither).
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
		if i < 0 || s.pendNeedsDecision(k, i) {
			return
		}
		s.takePending(k, i)
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
