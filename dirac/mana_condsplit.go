package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Condition-split mana source (kop.Ability.SplitGate: River of Tears'
// "{T}: Add {U}. If you played a land this turn, add {B} instead"; Gemstone
// Caverns' "{T}: Add {C}. If this has a luck counter on it, instead add one
// mana of any color"). gorge's payment planner defers the source
// (pay.PaymentPlanShapeTierOf: source:conditional), so the census never lists
// it and autopay never taps it; only a potential play's prefix search or the
// manual window does. That is the kernel's Taps-deferred route
// (mana_tapsdefer.go), so the source joins it through planDeferred.
//
// Two masks, one gate. The census / relaxed unit is gorge's condSplitLiveSub
// fold (rules mana_condsplit.go): the root's production always, the arm's
// beside it while the arm's gate holds on the live board (the kernel can
// always decide the gate). The activation makes the root's mana while the gate
// fails and the arm's, only, while it holds (CR 605.1a, 106.1: the root is
// gated EQ0, the sub GE1): a lucky Gemstone asks one colour of WUBRG and never
// makes {C}; River after a land drop makes {B} with no ask.
//
// The card's planner views (varPC) are [0] the root's alone (gate off), [1] the
// census unit's (gate on), [2] the activation's (gate on: the arm's alone).
// manaPCGen returns the activation's view, the potential walks (availWalk /
// availHyp) the census one, and addTapsDeferUnits prices a gap by the census
// one (splitCensusPC). The gate has one home, splitGateOn.

// DebugNoCondSplitGate forces the arm's gate off (the lockstep's mutation test
// flips it to prove the pool exercises the live-gate arm).
var DebugNoCondSplitGate bool

// prepManaSplit builds card c's three views when its first mana ability is a
// Condition-split one (prepareGen, once per card).
//
//go:noinline
func prepManaSplit(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	j := int(pc.manaAb)
	if j < 0 || j >= len(c.Abilities) || g.has(id, gfManaVar) {
		return
	}
	a := &c.Abilities[j]
	if a.Kind != kop.AbMana || a.SplitGate == kop.SplitGateNone || a.AFlags&kop.AfLatent != 0 || !pc.manaTap {
		return
	}
	root, act, census := *pc, *pc, *pc
	census.manaCol = a.ManaColors | a.SplitCols
	act.manaCol = a.SplitCols
	g.setFlag(id, gfManaVar)
	r := g.smallFor(id)
	r.manaVar, r.splitAb = int16(len(g.varPC)), int8(j)
	g.varPC = append(g.varPC, root, census, act)
	g.anyCondSplit, g.anyTapsDefer = true, true
}

// splitAb is card c's Condition-split mana ability index, -1 none.
func (g *genPrep) splitAb(c CardID) int8 {
	if !g.anyCondSplit || g.flags[c]&gfManaVar == 0 {
		return -1
	}
	if r := g.small(c); r != nil {
		return r.splitAb
	}
	return -1
}

// splitGateOn: the arm's gate of the Condition-split source at sl holds on the
// live board (the source's counter of the kind, the controller's lands played
// this turn: gorge's ConditionMet on the sub, GE1).
func (s *State) splitGateOn(k *prep, sl Slot, a *kop.Ability) bool {
	if DebugNoCondSplitGate {
		return false
	}
	switch a.SplitGate {
	case kop.SplitGateCounter:
		return s.perms[sl].ctrN(s, int(a.SplitCtr)) >= 1
	case kop.SplitGateLands:
		// gorge's PlayerCountPropertyYou$LandsPlayed is Player.LandsPlayed, reset only
		// when the seat's own turn begins: on an opponent's turn it still holds the
		// count of the seat's last own turn (landsHeld), not CR 305.2's "this turn"
		pl := &s.pl[SeatIx(s.perms[sl].ctrl)]
		if s.perms[sl].ctrl == s.active {
			return pl.landsPlayed >= 1
		}
		return pl.landsHeld >= 1
	}
	return false
}

// splitDeferred: sl is a Condition-split mana source the planner defers.
func (s *State) splitDeferred(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyCondSplit {
		return false
	}
	return k.gen.splitAb(s.cardAt(sl)) >= 0 && !s.genLost(k, sl)
}

// splitView is sl's planner view: 0 the root's alone, 1 the census unit's
// (gate on), 2 the activation's (gate on).
func (s *State) splitView(k *prep, sl Slot, c CardID, census bool) *pcard {
	vi := int(k.gen.manaVar(c))
	if !s.splitGateOn(k, sl, &k.t.Cards[c].Abilities[k.gen.splitAb(c)]) {
		return &k.gen.varPC[vi]
	}
	if census {
		return &k.gen.varPC[vi+1]
	}
	return &k.gen.varPC[vi+2]
}

// condSplitView is manaPCGen's arm for a Condition-split card.
//
//go:noinline
func (s *State) condSplitView(k *prep, sl Slot, c CardID) *pcard {
	return s.splitView(k, sl, c, s.g.availX&(availWalk|availHyp) != 0)
}

// splitCensusPC is the relaxed census unit's facts of the Condition-split
// source at sl (rules condSplitLiveSub: the root's production, the arm's
// beside it while its gate holds).
func (s *State) splitCensusPC(k *prep, sl Slot) *pcard {
	return s.splitView(k, sl, s.cardAt(sl), true)
}

// splitWindowPC is the facts gorge's window census prices the source by
// (windowManaUnits reads the root ability alone, whatever the gate).
func (s *State) splitWindowPC(k *prep, sl Slot) *pcard {
	return &k.gen.varPC[k.gen.manaVar(s.cardAt(sl))]
}

// splitUsable: the Condition-split gap at sl can be activated now
// (scriptTapsDefer's shape checks for it).
func (s *State) splitUsable(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	return pm.flags&pfTapped == 0 && !s.activationSick(k, sl) && !s.genLost(k, sl) && !s.manaGranted(k, sl) &&
		!(k.gen.anyCantAct && s.actForbidden(k, sl, true))
}
