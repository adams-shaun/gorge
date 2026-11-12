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
	g.blkTaxStall = g.anyGoad || g.ix[mechAttacksIfAble] != arenaNone || g.blgOn || g.manaTaps
	for i := range t.Cards {
		g.blkTaxStall = g.blkTaxStall || blkTaxGap(&t.Cards[i])
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMustAttack {
				g.blkTaxStall = true
			}
		}
	}
}

// blkTaxGap reports whether card c holds a part the block window's source walk
// does not model: the walk is the attack window's, so exactly what atkTaxGap
// refuses. Over-refusing is safe; admitting a shape the window misprices is not.
func blkTaxGap(c *kop.CardDef) bool { return atkTaxGap(c) }

// blkTaxStallPair is pairRefusalN's StallBlkTax half: the text and the two
// sides, the cards with a block tax and the cards with a gap.
func blkTaxStallPair(t *kop.Table, g *genPrep) (string, func(*kop.CardDef, int) bool, func(*kop.CardDef, int) bool) {
	_, _, b := atkTaxStallPair(t, g)
	text := "a block tax beside an attack requirement, a can't-attack-alone creature, a Taps watcher, a land-type grant or a mana source of another shape: gorge's budget and payment window for that part are not modelled (combat_blocktax.go)"
	a := abilityOn(func(x *kop.Ability) bool { return x.Kind == kop.AbStatic && x.Static.Kind == kop.StatBlockTax })
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

// blkTaxSpent is the charge of the pairs already declared.
func (s *State) blkTaxSpent(k *prep) int {
	var pairs [2 * MaxPerm]blkPair
	n := 0
	for i, c := 0, s.blockPairs(&pairs); i < c; i++ {
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
			n, ok := s.blkTaxCharge(k, s.objs[a.Source()].slot, s.objs[a.Target()].slot)
			if !ok || n > budget || spent+n > budget {
				continue
			}
		}
		d.Options[j] = a
		j++
	}
	d.N = uint16(j)
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
	for {
		have := 0
		for _, v := range pool {
			have += int(v)
		}
		if have >= total {
			break
		}
		sl, col, n := s.atkTaxNext(k, p)
		if sl == NoSlot {
			return false
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.poolAdd(p, col, n)
		s.artTag(k, p, sl, col)
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
