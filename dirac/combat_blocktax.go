package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The block tax (kop.StatBlockTax): a printed S:Mode$ CantBlockUnless with a
// generic mana Cost$ (Archangel of Tithes, Qal Sisma Behemoth, Myr Prototype,
// Hipparion, Awesome Presence, Cowed by Wisdom; CR 509.1c, 509.1d). gorge
// (rules/attack_cost.go, combat.go askBlockers):
//
//   - prices each (blocker, attacker) pair, the sum over the statics in force
//     whose ValidCard$ admits the blocker and whose Attacker$ admits the
//     attacker, an absent spec being unconditional on that half
//     (blockPairCharge); a negative charge is unpriceable and the pair fails
//     closed;
//   - offers a charged pair only while the defender's mana reaches its charge
//     (blockChargeAffordable: for a generic charge, the pool and every
//     source's units, the quantity Decision.MaxSum publishes), and budgets
//     the declaration as Decision.MaxSum over each option's Value, the pair's
//     charge (no own-units inflation, unlike the attack tax);
//   - pays the declaration's charge as it is answered: from the pool, else
//     through a window posing one tap per ask (askNextBlockPay), which the
//     oracle answers with the first source listed, the order the kernel's
//     atkTaxNext walks (the same attackManaSources). A declaration the
//     sources cannot pay is declined: the empty DeclareBlockers, the taps
//     standing (declineBlockDeclaration).
//
// The kernel declares blockers one pick at a time, so the budget is re-derived
// at each pick from the picks already made, and Done pays the charge
// (blkTaxPay). The defender's source walk is the attack window's
// (atkTaxShape, atkTaxUnits, atkTaxNext; the defender holds no declared
// attacker): a table that also holds a part the walk does not model is refused
// before its game (StallBlkTax, blkTaxGap).

// StallBlkTax: a block tax beside a part the kernel's source walk does not
// model (blkTaxGap): gorge plays the table, the kernel refuses it at its start
// (initStall).
const StallBlkTax = 172

func init() { stallNames[StallBlkTax] = "blktax" }

// DebugBlkTaxFree (test only, TestBlockTaxFreeMutation): no block is taxed.
var DebugBlkTaxFree bool

// DebugBlkTaxNoFill (test only, TestFourSeatBlkTaxMenaceLockstep): a block
// pick's completion is not priced (blkTaxFillCost reads zero).
var DebugBlkTaxNoFill bool

// blkTaxOn reports whether the table carries a block tax.
func blkTaxOn(k *prep) bool { return k.gen != nil && k.gen.anyBlkTax }

// prepBlkTax sets genPrep.anyBlkTax when a card of t has a StatBlockTax
// static, and blkTaxStall when another part of the table is one the source walk
// does not model beside it.
func prepBlkTax(t *kop.Table, g *genPrep) {
	g.anyBlkTax = tableHasStatic(t, kop.StatBlockTax)
	if !g.anyBlkTax {
		return
	}
	r := tableReach(t)
	g.blkTaxStall = r.landGrantLive(t, g) || atkTaxTapsLive(t, g) ||
		g.anyMinBlk || g.anyAloneBlk || g.anySuspect ||
		// the arena masks, not anyLure/anyMustBlock: those are set before
		// prepArena fills g.ix, so they hold on every table
		g.ix[mechLure] != arenaNone || g.ix[mechMustBlocked] != arenaNone || g.ix[mechBlocksIfAble] != arenaNone
	for i := range t.Cards {
		g.blkTaxStall = g.blkTaxStall || blkTaxGap(&t.Cards[i])
	}
	g.blkTaxStall = g.blkTaxStall || r.mustAttackEmblem(t, true) || atkTaxGrantGap(g)
}

// blkTaxFloorPart reports whether card c can make a block floor or a block
// duty the block tax does not model: a MinMaxBlocker floor, can't-block-alone,
// a lure, must-be-blocked, blocks-if-able or a suspect. The option builders
// decide those rules without the defender's mana (blockerOptionsLure's owed,
// mbReach), while gorge's KBlockers completes a declaration within
// Decision.MaxSum. Menace (printed, granted, a static's, a counter's) is
// modelled: blkTaxFillCost prices a pick's cheapest completion against the
// budget, so a budget that pays a menace attacker's first blocker but not its
// second offers neither (CR 509.1c, 509.1d, 702.111b).
func blkTaxFloorPart(c *kop.CardDef) bool {
	if c.MinBlockers != 0 || c.AloneBlk || c.Lure || c.MustBeBlocked {
		return true
	}
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if blkTaxFloorStatic(a) {
			return true
		}
		if progHas(a, blkTaxFloorInstr) {
			return true
		}
	}
	return false
}

// blkTaxFloorStatic: a lure or must-block static.
func blkTaxFloorStatic(a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && (a.Static.Kind == kop.StatLure || a.Static.Kind == kop.StatMustBlock)
}

// blkTaxFloorInstr: an instruction making a block duty (must-be-blocked, a
// lure, blocks-if-able, a creature that blocks the resolving source if able)
// or a suspect (menace and can't block, CR 701.60c).
func blkTaxFloorInstr(in *kop.Instr) bool {
	if in.Op != kop.OpExt {
		return false
	}
	return in.Ext() == kop.ExtMustBeBlocked || in.Ext() == kop.ExtBlocksIfAble || in.Ext() == kop.ExtMustBlock || in.Ext() == kop.ExtSuspect
}

// blkTaxGap reports whether card c holds a part the block tax does not model:
// what the attack window's source walk refuses (atkTaxGap: the block window
// walks the same sources), and a block floor or duty (blkTaxFloorPart). A
// broad attack requirement ("attacks each combat if able": Juggernaut, Graaz)
// shapes only the attackers decision (CR 508.1d); the block window prices a
// blocker pair from the defender's sources alone (TestFourSeatBlkTaxMustAttackLockstep).
// Over-refusing is safe; admitting a shape the window misprices is not.
func blkTaxGap(c *kop.CardDef) bool {
	return atkTaxGap(c) || blkTaxFloorPart(c)
}

// blkTaxStallPair is pairRefusalN's StallBlkTax half: the text and the two
// sides, the cards with a block tax and the cards with a gap.
func blkTaxStallPair(t *kop.Table, g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	_, _, atkB := atkTaxStallPair(t, g)
	text := "a block tax beside an attack requirement, a can't-attack-alone creature, a Taps watcher, a land-type grant, a mana source of another shape, menace, a blocker floor or a block duty: gorge's budget, budgeted block completion and payment window for that part are not modelled (combat_blocktax.go)"
	a := abilityOn(func(x *kop.Ability) bool { return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatBlockTax })
	b := func(c *kop.CardDef, i int) bool {
		return atkB(c, i) || blkTaxFloorPart(c)
	}
	return text, a, b
}

// blkTaxCharge is the generic mana the blocker at b blocking the attacker at a
// is charged, and false when a matching static's charge is negative (gorge's
// unpriceable pair). A static's Filter admits the blocker (matched from its
// source and controller); a nil Filter2 admits every attacker.
func (s *State) blkTaxCharge(k *prep, b, a Slot) (n int, ok bool) {
	ok = true
	if DebugBlkTaxFree {
		return 0, true
	}
	s.eachStatic(k, func(st statSrc) bool {
		x := st.a
		if x.Kind != kop.AbStatic || x.Static.Kind != kop.StatBlockTax || !s.statCond(k, st) ||
			!s.matchPermGen(k, x.Static.Filter, b, st.ctrl, st.sl, layerFull) ||
			x.Static.Filter2 != 0 && !s.matchPermGen(k, x.Static.Filter2, a, st.ctrl, st.sl, layerFull) {
			return true
		}
		v := int(x.Static.A)
		if x.Static.StFlags&kop.StDynamic != 0 {
			v = s.evalV(k, x.Static.VA, s.ctxStatic(st))
		}
		if v < 0 {
			ok = false
		} else {
			n += v
		}
		return true
	})
	return n, ok
}

// manaBudget is player p's mana budget (gorge attackBudget): the pool and every
// source's units.
func (s *State) manaBudget(k *prep, p uint8) int {
	n := 0
	for _, v := range s.pl[SeatIx(p)].pool {
		n += int(v)
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		n += s.atkTaxUnits(k, m.first())
	}
	return n
}

// blkTaxAdmits reports whether gorge offers the pair (b, a) at all under a
// block tax: its charge is priceable and the defender's budget reaches it
// (admissiblePair's blockChargeAffordable). anyBlock reads it, since gorge
// poses no blockers decision when every pair is priced out.
//
//go:noinline
func (s *State) blkTaxAdmits(k *prep, b, a Slot) bool {
	n, ok := s.blkTaxCharge(k, b, a)
	return ok && (n == 0 || n <= s.manaBudget(k, s.blockDef()))
}

// blkTaxSpent is the charge of the pairs the defending seat has declared: an
// earlier defender's pairs were paid in its own declaration (gorge
// handleBlockers charges the one chosen declaration, CR 509.1d), not again.
func (s *State) blkTaxSpent(k *prep) int {
	var pairs [2 * MaxPerm]blkPair
	n := 0
	def := s.blockDef()
	for i, c := 0, s.blockPairs(&pairs); i < c; i++ {
		if s.perms[pairs[i].b].ctrl != def {
			continue
		}
		v, _ := s.blkTaxCharge(k, pairs[i].b, pairs[i].a)
		n += v
	}
	return n
}

// blkTaxFilter drops the block picks gorge's offer list and budget refuse: a
// charged pair the defender's mana cannot pay, and a pick whose charge the
// budget left by the declared picks cannot carry (the oracle's admissible over
// Decision.MaxSum). Done stays: the declared picks fit by construction.
//
//go:noinline
func (s *State) blkTaxFilter(k *prep, d *Decision) {
	budget := s.manaBudget(k, s.blockDef())
	spent := s.blkTaxSpent(k)
	j := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkBlock {
			b, at := s.objs[a.Source()].slot, s.objs[a.Target()].slot
			n, ok := s.blkTaxCharge(k, b, at)
			if !ok || n > budget || spent+n > budget {
				continue
			}
			if f := s.blkTaxFillCost(k, b, at, budget-spent-n); f < 0 || spent+n+f > budget {
				continue
			}
		}
		d.Options[j] = a
		j++
	}
	d.N = uint16(j)
	if j == 0 {
		// Done withheld and every pick priced out: a budgeted completion the
		// kernel does not model (blkTaxFloorPart refuses every table known to
		// reach it). Stall, never pose an empty decision.
		s.stall(StallBlkTax)
		d.Kind = DkNone
	}
}

// blkTaxFillCost is the least charge that completes the declaration with the
// extra block (xb blocks xa) added: each blocker still owed to a menace
// attacker (menaceDeficit2) taken by a distinct free blocker (blocksCompletable's
// pool) whose pair is priceable, the sum of those pairs' charges; -1 when no
// completion costs at most limit. gorge offers a pick only when some whole
// answer holding it fits Decision.MaxSum (the oracle's completable over
// admissible: blockFloorsMatchable prices the missing blockers the same way),
// so a budget that pays a menace attacker's first blocker but not its second
// offers neither. Zero when nothing is owed. A depth-first search over the
// deficits, bounded by the best found: a deficit is one entry per missing
// blocker, a handful on any board.
func (s *State) blkTaxFillCost(k *prep, xb, xa Slot, limit int) int {
	if DebugBlkTaxNoFill {
		return 0
	}
	var def [MaxPerm]Slot
	nd := s.menaceDeficit2(k, xa, NoSlot, &def)
	if nd == 0 {
		return 0
	}
	var free [MaxPerm]Slot
	nf := 0
	for bm := s.ctrlM[s.blockDef()]; bm.any(); bm = bm.rest() {
		if b := bm.first(); b != xb && s.blockerReady(k, b) {
			free[nf] = b
			nf++
		}
	}
	if nf < nd || nd > len(def) {
		return -1
	}
	var used [MaxPerm]bool
	best := limit + 1
	s.blkTaxFill(k, &def, nd, 0, &free, nf, &used, 0, &best)
	if best > limit {
		return -1
	}
	return best
}

// blkTaxFill extends blkTaxFillCost's search from deficit i, cost spent so
// far, lowering *best on a cheaper completion.
func (s *State) blkTaxFill(k *prep, def *[MaxPerm]Slot, nd, i int, free *[MaxPerm]Slot, nf int, used *[MaxPerm]bool, cost int, best *int) {
	if i == nd {
		*best = cost
		return
	}
	for j := 0; j < nf; j++ {
		if used[j] || !s.canBlock(k, free[j], def[i]) {
			continue
		}
		n, ok := s.blkTaxCharge(k, free[j], def[i])
		if !ok || cost+n >= *best {
			continue
		}
		used[j] = true
		s.blkTaxFill(k, def, nd, i+1, free, nf, used, cost+n, best)
		used[j] = false
	}
}

// blkTaxPay pays the declaration's block tax as Done is answered (CR 509.1d,
// 509.1g; gorge handleBlockers): the pool first, then the window's taps until
// the pool covers the charge, then the charge from the pool (generic, C W U B R
// G first: payFromPool). It reports false when the sources run out first, the
// taps standing: gorge declines that declaration. The budget offers no such
// declaration, so only a bypassing one reaches it.
//
//go:noinline
func (s *State) blkTaxPay(k *prep) bool {
	p := s.blockDef()
	total := s.blkTaxSpent(k)
	if total == 0 {
		return true
	}
	pool := &s.pl[SeatIx(p)].pool
	pend, pendSl := -1, NoSlot
	for {
		have := 0
		for _, v := range pool {
			have += int(v)
		}
		if have >= total {
			break
		}
		sl, al := s.atkTaxNext(k, p)
		if sl == NoSlot {
			s.atkTaxPendingAsk(k, p, pend, pendSl)
			return false
		}
		pend, pendSl = s.atkTaxTap(k, p, sl, al), sl
	}
	for total > 0 {
		c := kop.ManaCost{Generic: uint8(min(total, 255))}
		total -= int(c.Generic)
		s.payFromPool(p, &c, 0)
	}
	return true
}

// dropBlockDeclaration abandons the declaration being answered: every declared
// pair is forgotten (gorge emits the empty DeclareBlockers).
func (s *State) dropBlockDeclaration(k *prep) {
	var pairs [2 * MaxPerm]blkPair
	for i, c := 0, s.blockPairs(&pairs); i < c; i++ {
		b, a := pairs[i].b, pairs[i].a
		s.forgetBlocked(b, a)
		if s.perms[b].blocking == NoSlot {
			s.perms[b].flags &^= pfBlocking
		}
		s.perms[a].nblock = 0
		s.touchSl(a)
	}
}
