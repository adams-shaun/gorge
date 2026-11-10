package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// KC10: conditional enters-tapped lands (ConditionPresent$ +
// ConditionCompare$ on a self ETB Tap replacement). The lowering is kgen's;
// the kernel already evaluates Repl.Cond through condEval with the entering
// object removed from occ/ctrlM. debugEntersTappedMutate is the test-only mutation
// switch (0 none, 1 the condition always holds, 2 evaluate it with the
// entering object still on the battlefield); TestConditionalEntersTappedMutation requires the
// lockstep to diverge under each.
var debugEntersTappedMutate uint8

// SetDebugEntersTappedMutate sets the mutation switch; tests only.
func SetDebugEntersTappedMutate(m uint8) { debugEntersTappedMutate = m }

// cold helpers for the two resolution-cost pay kinds.
//
// PaySubCounter (gorge trigger_cost_subcounter.go / unless_cost.go) removes
// the resolving ability's own counters and is settled in interp_ask.go; it
// needs no helper here.
//
// PayTapX is a printed trigger's own literal tapXType cost (gorge
// cumulative.go:1386, triggered_literal_tap.go): the tap election IS the
// payment, one permanent per answer, each tapping with the Tap events every
// other cost tap takes (cast.go payPicks' pkTapX arm).
func (s *State) tapCostPerm(k *prep, sl Slot, o Obj) {
	if s.perms[sl].flags&pfTapped != 0 {
		return
	}
	s.perms[sl].flags |= pfTapped
	s.touch(o)
	if k.gen != nil && s.subs[kop.TrigTaps].any() {
		s.fireTaps(k, sl)
	}
	s.markTapped(k, sl)
}

// a gated mana ability (IsPresent$ on a mana ability; gorge
// pay.ManaActivationGateHolds, applied wherever the mana abilities are
// listed: the priority offer, the payment window and the planner census, so
// a gated-off ability is absent from all three). The kernel's source view is
// manaPCGen; a gated card rides its variant arm: varPC[vi] is the gate-off
// view (the ungated ability alone, or no mana for a card whose only mana
// ability is gated) and varPC[vi+1] the gate-on view (the kmana2 union of a
// Verge, or the single ability). The gate is the ability's Trig.Cond (a
// CondPresent, "you" = the source's controller) read only here, per call.

// prepManaGate builds card c's two views (prepareGen, once per card).
//
//go:noinline
func prepManaGate(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	gj, n := -1, 0
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		n++
		if a.Trig.Cond != 0 && gj < 0 {
			gj = j
		}
	}
	if gj < 0 || g.has(id, gfManaVar) {
		return
	}
	off, on := noManaPC, *pc
	if n > 1 {
		// a Verge: the first ability alone while the gate is off
		off = *pc
		off.manaMulti = false
		for j := range c.Abilities {
			if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
				off.manaCol = a.ManaColors
				break
			}
		}
		if pc.painCol != 0 {
			// Spire of Industry: {C}, then a gated {T}, Pay 1 life: any
			// colour; the gate-on view is Starting Town's (mana_pain.go),
			// the gate-off view {C} with no painful alternative
			off.painCol, off.painN, off.painLife = 0, 0, false
		} else if u, ok := gatedChoiceUnion(c, gj, off); ok && !on.manaMulti && debugEntersTappedMutate != 6 { // 6: TestMirrexLockstepMutation
			// Mirrex and the MSH basic-land cycle:
			// {C}, then a gated choice of colours; gorge's census unit is
			// the {C} alternative then one per colour, the kernel's union
			// C|colours; the window activates ability 0 ({C})
			on.manaCol, on.manaMulti = u, true
		}
		if gatedLargerPair(c, gj, pc) && debugGatedPair != 1 {
			// Whisperer of the Wilds: {T}: Add {G}, then a gated {T}: Add
			// {G}{G}. The unit's two alternatives differ in amount; the
			// window answers ability 0 (manaN stays the {G}), the planner
			// and the affordability bounds read the larger (wideN)
			on.wideN, on.wideCol = c.Abilities[gj].ManaN, c.Abilities[gj].ManaColors&kop.ColAny
			g.anyGatePair = true
		}
		if on.manaMulti && bits.TrailingZeros8(uint8(off.manaCol)) != bits.TrailingZeros8(uint8(on.manaCol)) {
			g.anyVergeOrder = true
		}
	}
	g.setFlag(id, gfManaVar|gfManaGate)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	g.varPC = append(g.varPC, off, on)
}

// manaGateView is manaPCGen's arm for a gated card: the gate-on view while
// the gate holds, else the gate-off one.
//
//go:noinline
func (s *State) manaGateView(k *prep, sl Slot, c CardID, vi int16) *pcard {
	cd := &k.t.Cards[c]
	on := false
	if k.gen.flags[c]&gfFiltFee != 0 {
		if s.filtFee(k, sl) == 0 {
			return &k.gen.varPC[int(vi)+1]
		}
		return &k.gen.varPC[int(vi)]
	}
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbMana && a.Trig.Cond != 0 && a.AFlags&kop.AfLatent == 0 {
			on = s.condEval(k, a.Trig.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl})
			if debugEntersTappedMutate == 5 { // TestConditionalEntersTappedMutation (e): the gate is always on
				on = true
			}
			break
		}
	}
	if on {
		return &k.gen.varPC[int(vi)+1]
	}
	return &k.gen.varPC[int(vi)]
}

// part A: a gated Verge whose second mana ability is the
// LOWER colour in WUBRG order (Bleachbone, Riverpyre, Wastewood, Hushwood).
// gorge's payment window and its failed ward payment activate a two-ability
// source through "Choose a mana ability of <name>" (mana_activation.go
// activateManaFor); the oracle answers the first option, ability 0, and a
// one-colour ability carries no colour symbol. For an in-order Verge ability
// 0 is the lowest colour, which the kernel always took; for these four it is
// the higher one. The gate-off view prepManaGate builds is the ability 0
// alone, so its colour is the answer.

// vergeAbility0Col returns the colour of the gated mana source at sl's
// ability 0, and false when sl is not a gated source.
func (s *State) vergeAbility0Col(k *prep, sl Slot) (int, bool) {
	c := s.cardAt(sl)
	if k.gen.flags[c]&(gfManaGate|gfManaRefl) == 0 {
		return 0, false // a reflected source's view 0 is its ability 0, {C} (mana_reflected.go)
	}
	vi := k.gen.manaVar(c)
	if vi < 0 {
		return 0, false
	}
	if k.gen.flags[c]&gfManaRefl == 0 && gatedChoiceFlat(k.gen.varPC[int(vi)+1].manaCol) {
		return 0, false // a gated {C}-plus-colours choice unit (a reflected source's views are {C}|m, not this): gorge's colour ask (mana_gatedchoice.go)
	}
	return bits.TrailingZeros8(uint8(k.gen.varPC[vi].manaCol)), true
}

// windowTapHeldOK is windowTapHeld reporting whether the window paid the
// cost. A power-up activation's window can run dry: gorge offers it on the
// printed cost's plan, then the window's Verge ability-0 answers (and the
// clamp) leave a coloured pip unpaid, and it aborts the activation.
func (s *State) windowTapHeldOK(k *prep, p uint8, cost *kop.ManaCost) bool {
	ok, held := s.windowTap(k, p, cost, true)
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
	return ok
}

// debugGatedPair is the lockstep mutation switch of TestWhispererGatedPairMutation:
// 1 leaves the gated larger alternative out of the gate-on view (the unit
// makes one mana), 2 leaves the planner's pair arm out (the unit stays one
// {G} type beside the larger affordability bound). Zero in every real run.
var debugGatedPair uint8

// SetDebugGatedPair sets the mutation switch; tests only.
func SetDebugGatedPair(m uint8) { debugGatedPair = m }

// gatedLargerPair reports card c's gated ability gj as a larger alternative of
// the first mana ability's colour: two plain {T} abilities of one fixed colour,
// the gated one making more (Whisperer of the Wilds, Fanatic of Rhonas).
// gorge's census lists the gate-on source as one unit with both alternatives
// (pay.appendUnitAlternatives) and the planner takes the one that ranks best
// per payment; the kernel's unit is a restrPairUnit pair without the
// restricted float.
func gatedLargerPair(c *kop.CardDef, gj int, pc *pcard) bool {
	if pc.manaAb < 0 || int(pc.manaAb) == gj {
		return false
	}
	a0, ag := &c.Abilities[pc.manaAb], &c.Abilities[gj]
	n := 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			n++
		}
	}
	return n == 2 && a0.AFlags == 0 && ag.AFlags == 0 && a0.ManaRestrict == 0 && ag.ManaRestrict == 0 &&
		a0.ManaColors == ag.ManaColors && bits.OnesCount8(uint8(ag.ManaColors&0x3f)) == 1 &&
		a0.Cost.Flags == kop.CostTap && ag.Cost.Flags == kop.CostTap && max(ag.ManaN, 1) > pc.manaN && !ag.ManaEach && !a0.ManaEach
}

// srcYield is the mana a source's view can make in one tap for an
// affordability bound: the larger alternative of a gated pair (wideN) holds
// the census one's. The payment window still pays manaN (ability 0).
func srcYield(pc *pcard) uint8 { return max(pc.manaN, pc.wideN) }

// gatePairPotential is what a gate-on pair view adds to the potential pool
// beside its census manaN: gorge's potentialManaFor admits each listed mana
// ability of a source on its own and sums them, and with the gate on both
// {T}: Add {G} and {T}: Add {G}{G} are listed (Whisperer: 3 {G}, not the
// planner's max of 2). Zero for any other view. A gate-on pair view reaches the
// potential walk with wideN set (manaPCGen never returns a restricted record
// view); a dynamic pair view (mana_dynpair.go) does too but the walk reads it
// as open before this.
func gatePairPotential(pc *pcard) uint8 { return pc.wideN }
