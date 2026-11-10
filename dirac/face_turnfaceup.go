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
	if kw && cost.X > 0 {
		return s.turnUpMaxX(k, p, cost, sp, sl) >= 0
	}
	return s.specialAffords(k, p, cost, sp, sl, 0)
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
