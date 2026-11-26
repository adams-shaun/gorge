package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// R:Event$ TurnFaceUp replacements (CR 702.37e, 708.6,
// 614.17a). Cold: every hot site is guarded by genPrep.anyTurnUpRepl.
//
// gorge matches the permanent being turned up in its pre-flip (face-down)
// state, reads ValidCard$ in the replacement source's frame, and gates on
// PlayerTurn$ (the source's controller is the active player):
//   - a Layer$ CantHappen replacement (ReplCantTurnUp) makes the special
//     action illegal -- never offered, nothing paid (turnFaceUpCantHappen,
//     replacement_phase.go; legal_walk_turnup.go);
//   - a ReplaceWith$ body (ReplTurnUpProgram) runs after the cost is paid and
//     before the turn-up event folds (applyTurnFaceUpReplacements), so its
//     counters land on the still face-down permanent.

// DebugTurnUpReplOff skips turnUpBlocked (the mutation: the kernel offers
// and performs a turn-up gorge refuses).
var DebugTurnUpReplOff bool

// DebugTurnUpReplStats counts, for the tests: TurnUpBlockedHeld the offers
// withheld by a ReplCantTurnUp that would otherwise have been made
// (TurnUpHeldCond: by one with a Repl.Cond, a Watchdog, and by no other;
// TurnUpHeldPlain: by one without, an Aura, and by no other: a hold two
// sources share would pass without the lockstep seeing either), and
// TurnUpReplCtrs the turn-ups whose ReplTurnUpProgram ran.
var (
	DebugTurnUpReplStats bool
	TurnUpBlockedHeld    int
	TurnUpHeldCond       int
	TurnUpHeldPlain      int
	TurnUpReplCtrs       int
	turnUpWhy            uint8 // stats mode: 1 = a ReplCantTurnUp with a Cond blocked, 2 = one without
)

// turnUpBlocked reports whether a battlefield replacement with ReplCantTurnUp
// forbids turning the face-down permanent at sl face up: the source has not
// lost its abilities, Repl.Filter matches sl read in the source's frame and
// Repl.Cond holds for the source's controller.
//
//go:noinline
func (s *State) turnUpBlocked(k *prep, sl Slot) bool {
	if DebugTurnUpReplOff {
		return false
	}
	var why uint8
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.genLost(k, src) {
			continue
		}
		sp := &s.perms[src]
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplCantTurnUp {
				continue
			}
			if a.Repl.Cond != 0 && !s.condEval(k, a.Repl.Cond, evalCtx{src: sp.obj, srcSl: src, ctrl: sp.ctrl}) {
				continue
			}
			if s.matchPermGen(k, a.Repl.Filter, sl, sp.ctrl, src, layerFull) {
				if !DebugTurnUpReplStats {
					return true
				}
				if a.Repl.Cond != 0 {
					why |= 1
				} else {
					why |= 2
				}
			}
		}
	}
	turnUpWhy = why
	return why != 0
}

// turnUpRepls runs the turned permanent's own ReplTurnUpProgram abilities
// (it is the source, RefReplaced is itself) while it is still face down.
// The program goes through exec, not runReplProg: that loop runs execGen
// alone, which finishes the as-enters ops but hands OpPutCounter to exec.
func (s *State) turnUpRepls(k *prep, sl Slot, o Obj) {
	if s.ovrLoses(k, sl) {
		return // gorge replacementFace: a lose-all effect still silences the face-down permanent's own replacement
	}
	c := s.cardAt(sl)
	cd := &k.t.Cards[c]
	saveRep, saved := s.g.replaced, s.res
	s.g.replaced = o
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplTurnUpProgram {
			continue
		}
		if a.Repl.Cond != 0 && !s.condEval(k, a.Repl.Cond, evalCtx{src: o, srcSl: sl, ctrl: s.perms[sl].ctrl}) {
			continue
		}
		if DebugTurnUpReplStats {
			TurnUpReplCtrs++
		}
		if !saved.active {
			sp := &s.perms[sl]
			s.res = resCtx{e: stackEntry{kind: seAbility, obj: o, srcVer: s.objs[o].ver, card: c, ab: uint8(i), ctrl: sp.ctrl, trigObj: NoObj}}
			s.res.e.tgt = noTargets
		}
		for j := range a.Prog {
			s.exec(k, &a.Prog[j])
		}
	}
	s.res = saved
	s.g.replaced = saveRep
}

// faceDownWard reports whether the face-down permanent at sl has ward {2}
// as a base keyword (CR 702.168b, 701.58b): one cast face down with
// Disguise, or a cloaked one. A MANIFESTED card has no abilities, a printed
// Disguise included (CR 701.40a; gorge BaseKeywords), so a manifested
// Bubble Smuggler has no ward.
func (s *State) faceDownWard(k *prep, sl Slot) bool {
	return s.cloaked(sl) || !s.manifested(sl) && k.t.Cards[s.cardAt(sl)].MorphFamily == kop.MorphDisguise
}

// turnUpHybOK reports whether a turn-up cost is one the floating pool prices:
// a plain cost or one with ordinary hybrid pips (Crowd-Control Warden's
// Disguise {3}{G/W}{G/W}). A twobrid or Phyrexian pip needs an announcement
// (a life or generic alternative) the special action does not pose here.
func turnUpHybOK(cost *kop.ManaCost) bool {
	return cost.Hyb[0] == 0 || cost.Hyb[0] != kop.HybTwobrid && !cost.Phyrexian()
}

// turnUpHybResolve resolves cost's hybrid pips against the floating pool the
// way the payment search does (hybWitness: the first resolution the pool
// affords), so the payment spends plain pips. The caller proved
// specialAffords (mana_specialaction.go).
func (s *State) turnUpHybResolve(p uint8, cost kop.ManaCost, extra int) kop.ManaCost {
	pl := &s.pl[SeatIx(p)]
	var a avail
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<c, pl.pool[c])
		}
	}
	return a.hybWitness(cost, extra)
}

// turnUpMaxX is the largest X the special action's mana — the floating pool
// with the restricted batches its restriction admits — pays for a keyword
// turn-up (gorge turnUpXAsk's bound, no reductions), -1 when none.
func (s *State) turnUpMaxX(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot) int {
	pl := &s.pl[SeatIx(p)]
	bound := int(cost.Generic) + int(cost.X) + 1
	for c := 0; c < kop.NumManaKinds; c++ {
		bound += int(pl.pool[c])
	}
	if m := s.restrPoolSpecial(k, p, sp, sl); m != 0 {
		for i := range s.rb[SeatIx(p)] {
			b := &s.rb[SeatIx(p)][i]
			if b.kind == 0 {
				break
			}
			if m&(1<<i) != 0 {
				bound += int(b.units())
			}
		}
	}
	bound = min(bound, 255)
	best := -1
	for x := 0; x <= bound; x++ {
		if s.specialAffords(k, p, cost, sp, sl, x) {
			best = x
		}
	}
	return best
}

// turnUpOffered: the special action's mana pays the turn-up (a keyword cost
// with {X} at some announced X).
func (s *State) turnUpOffered(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, kw bool) bool {
	if kw && !s.turnUpKwPayable(k, p, sl) {
		return false // face_morphlife.go, act_revealpick.go: the keyword's PayLife<N> or Reveal<1/Spec>
	}
	if kw && cost.X > 0 {
		return s.turnUpMaxX(k, p, cost, sp, sl) >= 0
	}
	return s.specialAffords(k, p, cost, sp, sl, 0)
}

// turnUpListed: the keyword turn-up is offered from the pool (turnUpOffered),
// or from the pool and the untapped sources together at the smallest {X}:
// gorge lists it as a witnessed potential play (rules' turn_face_up
// witness, priced at the smallest legal announcement), so the kernel offers
// the same action without a float. The action's turnFaceUp then taps that
// witness's sources. A manifested or cloaked card's printed-cost procedure
// stays pool-only, matching the oracle's keyword-only projection.
func (s *State) turnUpListed(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, kw bool, mode uint8) bool {
	if s.turnUpOffered(k, p, cost, sp, sl, kw) {
		return true
	}
	if !kw {
		// A manifested or cloaked card's printed-cost procedure is not a
		// potential play yet (the oracle projects only the keyword procedures):
		// the pool-only offer is the one gorge's ordinary walk has.
		return false
	}
	if !s.turnUpKwPayable(k, p, sl) {
		// face_morphlife.go: the keyword's PayLife<N> is not payable, so no
		// tap of the sources can make the action legal (turnUpOffered's gate)
		return false
	}
	c0 := cost.AtX(0)
	return s.turnUpTapPays(k, p, &c0, sp, sl, mode) || s.storeCensusPays(k, p, &c0)
}

// turnUpTapPays: the untapped sources (or a restricted, rider, planner or
// filter route) pay the keyword turn-up's cost at the smallest {X}.
func (s *State) turnUpTapPays(k *prep, p uint8, c0 *kop.ManaCost, sp kop.SpecialMask, sl Slot, mode uint8) bool {
	return s.sourcesPay(k, p, c0) || s.turnUpRestrPays(k, p, c0, sp, sl) || s.turnUpRiderPays(k, p, c0) || s.turnUpPlannerPays(k, p, c0) || s.turnUpFilterPays(k, p, sl, mode) || s.turnUpTapXPays(k, p, sl, mode) || s.turnUpDeferPays(k, p, sl, mode)
}

// turnUpStoreOnly: only an idle storage battery's open production prices the
// keyword turn-up (turnUpListed's last arm), no source the planner taps does.
// The action's payment activates that battery first (storeTurnUpTap).
func (s *State) turnUpStoreOnly(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, mode uint8) bool {
	c0 := cost.AtX(0)
	return !s.turnUpTapPays(k, p, &c0, sp, sl, mode) && s.storeCensusPays(k, p, &c0)
}

// turnUpRiderPays: gorge's potential walk counts a depletion land's whole
// production (Sandstone Needle's {R}{R}, potentialGapCost) beside the
// planner's sources, so the keyword turn-up is a potential play the sources
// and that rider's activation pay together. The window's walk prices it on a
// fork (windowAffords), as a cast's gap prefix is.
//
//go:noinline
func (s *State) turnUpRiderPays(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil {
		return false
	}
	ok, _ := s.windowAffords(k, p, cost)
	return ok
}

// turnUpPlannerPays: the payment planner plans the turn-up's cost beside the
// floating pool (a Combo Any source's activation among its census units,
// mana_comboanyplan.go), the census neither sourcesPay's plain walk nor the
// window's gap kinds cover. gorge's turn-up witness is that planner's
// (potential_plan.go optTurnFaceUp: planComposedCost under the procedure's
// special action, which priced Exploding Barrel's any-colour activation into
// a {5}{G}{G} morph turn-up), so the offer matches it. The probe is dry
// (pk.dry): it selects the plan without tapping, and the action's payment
// re-taps through tapForTurnUp, the planner that plans the same units.
//
//go:noinline
func (s *State) turnUpPlannerPays(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil || cost.Hyb[0] != 0 {
		// a hybrid pip's printed alternative stays the availability routes'
		// (gorge's own hybrid witness is planHybridCost's, not this one's)
		return false
	}
	var pk planner
	pk.dry = true
	s.tapPlanNeed(k, p, cost, 0, NoObj, false, false, &pk)
	return pk.needT > 0 && pk.found
}

// turnUpRiderOnly: neither the planner's sources nor a restricted source pay
// the turn-up's cost, only the window's walk with a depletion land does, so
// the planner has no plan to tap (tapForTurnUp would find none).
//
//go:noinline
func (s *State) turnUpRiderOnly(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot) bool {
	return !s.sourcesPay(k, p, cost) && !s.turnUpRestrPays(k, p, cost, sp, sl) && s.turnUpRiderPays(k, p, cost)
}

// tapTurnUpRiders taps the sources the window's walk takes for cost, the
// depletion lands' activations among them, once the planner's sources fall
// short (turnUpRiderPays).
//
//go:noinline
func (s *State) tapTurnUpRiders(k *prep, p uint8, cost *kop.ManaCost) {
	s.windowTapWalk(k, p, cost, cost, false, false, false, false, false, false, false, false)
}

// turnUpRestrPays: the turn-up's potential witness counts the restricted mana
// its Static.* restriction admits (CR 106.6, 116.2b): p's floating batches and
// its untapped restricted sources beside the ordinary sources (Tin Street
// Gossip's {R}{G} "to turn creatures face up"), as gorge's potential bound
// counts the floating pool whole and the source's production.
//
//go:noinline
func (s *State) turnUpRestrPays(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot) bool {
	if k.gen == nil || !k.restrSrc && !s.rbAny(SeatIx(p)) {
		return false
	}
	var av avail
	s.availFor(k, p, &av)
	if m := s.restrPoolSpecial(k, p, sp, sl); m != 0 {
		s.restrAvailAdd(p, m, &av)
		if av.affords(cost, pipMaskOf(cost), 0) {
			return true
		}
	}
	return k.restrSrc && s.restrSrcAffords(k, p, payDesc{class: pdSpecial, src: NoSlot, sp: sp, tgt: sl}, cost, &av, NoSlot, 0)
}

// tapForTurnUp is tapFor for the turn-up's witnessed sources: the planner
// taps the restricted sources the special action may spend (payDescNow reads
// the parked special and permanent).
//
//go:noinline
func (s *State) tapForTurnUp(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, o Obj) {
	ctl, left, f := s.ctl, s.g.pickLeft, s.g.pickF
	s.ctl, s.g.pickLeft, s.g.pickF = ctlTurnUpTap, uint8(sp), uint16(o)
	s.tapFor(k, p, cost, 0, NoObj)
	s.ctl, s.g.pickLeft, s.g.pickF = ctl, left, f
}

// applyTurnUpX takes the announced X of a parked keyword turn-up.
func (s *State) applyTurnUpX(k *prep, a Action) {
	p, mode, o := s.g.pickP, s.g.pickZ, Obj(s.g.pickF)
	s.ctl = ctlPriority
	s.settleTurnUp(k, o, p, mode, int(a.Mode()))
}

// turnUpXOptions poses the X ask of a parked keyword turn-up.
func (s *State) turnUpXOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChX, s.g.pickP
	for x := 0; x <= int(s.g.pickLeft); x++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
	}
}
