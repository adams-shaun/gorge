package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the aggregate taps.
// Cold and out of line; reached behind genPrep.anyTapPower / anySaddle
// (a generated table without Crew, Saddle or Mossbridge Troll never takes
// these paths) or from execCold6's default branch.
//
// gorge (pin ec2218895): K:Crew / K:Saddle expand into one activated
// ability whose cost is tapXType<Any/Creature.Other+withTotalPowerGE<N>>
// (cards/kw_crew.go, kw_saddle.go). The floor is CostPart.MinPower
// (rules/cost/parse.go stripGroupPowerFloor). The offer needs one
// candidate and the candidates' summed power >= N (rules/pay/castparts.go);
// the payment is one KChoose "tapcost", Min 1, Max the candidates, MinSum
// N, each option's Value the candidate's current power (rules/cast_asks.go
// tapPermanentCostAsk). The kernel poses it as sequential picks closed by
// Done, as the oracle walks a multi-pick decision (oracle/gorge.go
// repose): a candidate is offered while the answer stays completable,
// Done once one is picked and the total reaches N, and the answer ends
// when every candidate is picked. Saddled (CR 702.171b) is gorge's
// Object.SaddledTurn: perm.cseen bit cseenSaddled here.

// cseenSaddled: the permanent is saddled (OpSaddle; gorge SaddledTurn ==
// the turn). perm.cseen bit 5 (bits 0-4 counters, 6-7 cseenSkipUntap*).
const cseenSaddled uint8 = 1 << 5

// prepCrewSaddle sets the prep bits: some ability costs CostTapPower, some
// program saddles (OpSaddle).
func prepCrewSaddle(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			g.anyTapPower = g.anyTapPower || a.Cost.Flags&kop.CostTapPower != 0
			for _, in := range a.Prog {
				// a Case's Solved (C = 1) is never cleared as a turn begins
				g.anySaddle = g.anySaddle || in.Op == kop.OpSaddle && in.C == 0
				if in.Op == kop.OpSaddle && in.C == 1 {
					g.anySolve = true
				}
			}
		}
	}
}

// tapPowerPayable: the CostTapPower part of ability a of the permanent at
// src can be paid -- at least one candidate, and their summed current
// power reaches the floor (gorge castparts.go: avail > 0, floorSum >=
// MinPower).
//
//go:noinline
func (s *State) tapPowerPayable(k *prep, a *kop.Ability, src Slot) bool {
	p := s.perms[src].ctrl
	tp := a.Cost.Flags & (kop.CostCrew | kop.CostSaddle)
	n, sum := 0, 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped != 0 || sl == src && a.Cost.Flags&kop.CostTap != 0 || !s.matchPerm(k, a.Cost.Filter, sl, p, src) {
			continue
		}
		n++
		sum += s.power(k, sl)
		if tp != 0 {
			sum += s.tapPowerValue(k, sl, a.Cost.Flags, false, src)
		}
	}
	return n > 0 && sum >= int(a.Cost.N)
}

// tapPowerOptions poses the next pick of the entry's CostTapPower part
// (bsTapCost, before any CostTapX pick). false: the part is paid (or the
// entry has none) and the build moves on.
//
//go:noinline
func (s *State) tapPowerOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seAbility || s.bld().donePicks&pkTapX != 0 {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&kop.CostTapPower == 0 {
		return false
	}
	src := s.srcSlot()
	floor := int(pc.N)
	tp := pc.Flags & (kop.CostCrew | kop.CostSaddle)
	sum, picked := 0, 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkTapX {
			continue
		}
		picked++
		if ob := &s.objs[s.bld().picks[i]]; ob.zone == kop.ZoneBattlefield {
			sum += s.power(k, ob.slot)
			if tp != 0 {
				sum += s.tapPowerValue(k, ob.slot, pc.Flags, false, src)
			}
		}
	}
	var cand [MaxPerm]Slot
	var pw [MaxPerm]int
	nc, rest := 0, 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if s.picked(s.perms[sl].obj) || s.perms[sl].flags&pfTapped != 0 ||
			sl == src && pc.Flags&kop.CostTap != 0 || !s.matchPerm(k, pc.Filter, sl, e.ctrl, src) {
			continue
		}
		cand[nc] = sl
		pw[nc] = s.power(k, sl)
		if tp != 0 {
			pw[nc] += s.tapPowerValue(k, sl, pc.Flags, false, src)
		}
		if pw[nc] > 0 {
			rest += pw[nc]
		}
		nc++
	}
	valid := picked > 0 && sum >= floor
	if nc == 0 || int(s.bld().pickN) >= len(s.bld().picks) {
		// every candidate picked: gorge's answer is complete at Max picks
		// (no Done); a pick past the record's capacity stalls
		if nc > 0 {
			s.capFull(StallTapPick)
		}
		s.tapPowerDone()
		return false
	}
	d.N = 0
	for i := 0; i < nc; i++ {
		// completable: this pick plus every other positive candidate
		// reaches the floor (gorge's repose: admissible and completable)
		others := rest
		if pw[i] > 0 {
			others -= pw[i]
		}
		if sum+pw[i]+others >= floor {
			d.add(MakeAction(AkChoose, uint8(s.perms[cand[i]].obj), 0, 0))
		}
	}
	if valid {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	if d.N == 0 {
		// nothing completes the answer (the offer gate makes this
		// unreachable): end the part as gorge's repair would
		s.tapPowerDone()
		return false
	}
	s.bld().pickK = pkTapX
	d.Kind, d.Sub, d.Player = DkChoose, ChCostTap, e.ctrl
	return true
}

// tapPowerDone closes the CostTapPower pick: the picks stay recorded as
// pkTapX taps (payPicks taps them with the cost).
func (s *State) tapPowerDone() {
	s.bld().donePicks |= pkTapX
	s.bld().pickK, s.bld().pickLeft = 0, 0
}

// saddle is OpSaddle: the battlefield permanent Obj becomes saddled.
func (s *State) saddle(k *prep, in *kop.Instr) {
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		was := s.perms[sl].cseen&cseenSaddled != 0
		s.perms[sl].cseen |= cseenSaddled
		s.touchSl(sl)
		if in.C == 0 {
			s.fireBecomesSaddled(k, sl, was)
		}
	}
}

// unsaddleAll ends every saddled designation as a turn begins (gorge
// TurnChange: SaddledTurn = 0).
//
//go:noinline
func (s *State) unsaddleAll(k *prep) {
	if DebugSaddleTgtNoClear { // TestSaddletgtMutation: never clear
		return
	}
	for m := s.occ; m.any(); m = m.rest() {
		if p := &s.perms[m.first()]; p.cseen&cseenSaddled != 0 && !(k.gen.anySolve && solves(k, s.cardAt(m.first()))) {
			p.cseen &^= cseenSaddled
			s.touch(p.obj)
		}
	}
}

// execCrewSaddle runs the ops (execCold6's default branch, cold). done=false means
// the op is not one of these.
//
//go:noinline
func (s *State) execCrewSaddle(k *prep, in *kop.Instr) (ok, done bool) {
	switch in.Op {
	case kop.OpSaddle:
		s.saddle(k, in)
		return true, true
	}
	return s.execEachFromSource(k, in)
}

// solves reports whether card c's programs solve it (OpSaddle C = 1): a
// Case keeps its Solved bit when a turn begins (unsaddleAll, reached only
// in a pool that saddles too).
func solves(k *prep, c CardID) bool {
	for i := range k.t.Cards[c].Abilities {
		for _, in := range k.t.Cards[c].Abilities[i].Prog {
			if in.Op == kop.OpSaddle && in.C == 1 {
				return true
			}
		}
	}
	return false
}
