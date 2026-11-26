package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PayLifeInsteadOf:B (K'rrik, Son of Yawgmoth's static: "For each {B} in a
// cost, you may pay 2 life rather than pay that mana", CR 601.2h; gorge
// engine/rules/mana.go PayLifeInsteadOfB). The grant is read live from the
// battlefield (the statics of a permanent p controls, as defenderAssigns
// walks them): no State bytes. A permanent that lost its abilities grants
// nothing (genLost). Every mana payment and every offer gate of the holder
// consults it, so a plain {B} pip is payable with 2 life anywhere.

// DebugNoPayLifeInsteadOfB is the lockstep mutation switch: the grant reads
// as absent. It must make the staticzones lockstep diverge
// (TestStaticZonesPayLifeMutation). Read in payLifeInsteadOfB only.
var DebugNoPayLifeInsteadOfB bool

// payLifeInsteadOfB reports whether p holds a PayLifeInsteadOf:B grant
// (gorge asEval(e).PayLifeInsteadOfB).
//
//go:noinline
func (s *State) payLifeInsteadOfB(k *prep, p uint8) bool {
	if DebugNoPayLifeInsteadOfB || k.gen == nil || !k.gen.anyPayLife {
		return false
	}
	for m := s.statM.and(s.ctrlM[p]); m.any(); m = m.rest() {
		sl := m.first()
		if s.genLost(k, sl) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(sl)]
		for i := range cd.Abilities {
			if ab := &cd.Abilities[i]; ab.Kind == kop.AbStatic && ab.Static.Kind == kop.StatPayLifeInsteadOfB {
				return true
			}
		}
	}
	return false
}

// payLifeGrantCost is the cost under a PayLifeInsteadOf:B grant: every plain
// {B} pip becomes a Phyrexian pip (kop.ManaCost.Tb, HybPhyrexian), so the
// cast announces each pip's face before its targets (gorge manaAsk's
// AnnounceGrantedBlackPips at PayIdx 0, CR 601.2h: each pip's payment is the
// payer's per-pip choice, the shape of a Phyrexian pip's). The converted
// pips ride the printed-Phyrexian machinery (mana_twobrid.go, mana_phyrexian.go),
// and their colour face — a plain {B} pip once it folds — is itself
// life-payable (gorge's ResolveManaWith bLifeOK arm: the folded face is
// indistinguishable from a printed one). A cost with a two-colour hybrid pip
// or a printed non-Phyrexian twobrid pip beside the {B} stays plain: the
// kernel's one Tb list cannot carry it beside them (gorge's lists can), so
// those costs are left to the afford and payment reads (bLifeTake) without
// the pip asks.
func payLifeGrantCost(c kop.ManaCost) kop.ManaCost {
	n := int(c.Pips[kop.B])
	if n == 0 {
		return c
	}
	if c.Hyb[0] != 0 && c.Hyb[0] != kop.HybTwobrid || c.Tb != 0 && c.Hyb[1] != kop.HybPhyrexian {
		return c
	}
	if c.TbN()+n > 5 {
		return c // the Tb list holds five pips
	}
	c.Hyb[0], c.Hyb[1] = kop.HybTwobrid, kop.HybPhyrexian
	for i := 0; i < n; i++ {
		c.Tb |= uint16(kop.B+1) << (3 * uint(c.TbN()))
	}
	c.Pips[kop.B] = 0
	return c
}

// payLifeCastCost is the hand cast's offer cost under the payer's
// PayLifeInsteadOf:B grant: the cost's {B} pips become Phyrexian pips
// (payLifeGrantCost), so the offer prices the faces the announcement will
// pose (gorge's composed cost carries the converted pips into the ask). The
// converted cost is written to *out; without the grant cost comes back as is.
func (s *State) payLifeCastCost(k *prep, p uint8, cost, out *kop.ManaCost) *kop.ManaCost {
	if k.gen == nil || !k.gen.anyPayLife || !s.payLifeInsteadOfB(k, p) {
		return cost
	}
	*out = payLifeGrantCost(*cost)
	return out
}

// castUnpayable reports that the hand cast of printed (cost after the cost
// statics, cost's plain price) is not offered: the grant's converted cost is
// refused (payLifeCastUnoffered) or neither the pool nor the pain payers pay it.
func (s *State) castUnpayable(k *prep, p uint8, c CardID, o Obj, printed kop.ManaCost, cost *kop.ManaCost, av, cav *avail) bool {
	return s.payLifeCastUnoffered(k, p, c, o, printed, av, cav) ||
		!s.castPaysUnderPain(k, p, c, o, cost, cav) && !s.instantTapsLethalPays(k, p, c, o, cost)
}

// payLifeWitnessCost is the cost the lowered witness plans for the spell being
// built: the printed {B} pips under the grant, and the pip-free mana half
// while the life pays every Phyrexian pip.
func (s *State) payLifeWitnessCost(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyPayLife {
		return cost
	}
	if s.cfg&cfgGorgeAutopay == 0 && s.bld().e.kind == seSpell && k.t.Cards[s.bld().e.card].Cost.Tb == 0 && s.payLifeInsteadOfB(k, p) {
		cost = payLifeGrantPlain(cost) // the lowered witness plans the printed {B} pips
	}
	if cost.Phyrexian() && s.bld().e.kind == seSpell && int(s.pl[p].life) >= 2*cost.TbN() {
		// gorge's witness funds only the pip-free mana half (pay.PlanManaHalf);
		// each pip takes its colour face or its two life in the window after
		// it, unless the life cannot pay them all (castPlanPipsOutcome)
		cost = phyPlanHalf(cost)
	}
	return cost
}

// payLifeCastUnoffered reports whether the grant converted printed's {B} pips
// and payLifeGrantOffered then refuses the hand cast.
func (s *State) payLifeCastUnoffered(k *prep, p uint8, c CardID, o Obj, printed kop.ManaCost, av, cav *avail) bool {
	if k.gen == nil || !k.gen.anyPayLife || payLifeGrantCost(printed).Tb == 0 || !s.payLifeInsteadOfB(k, p) {
		return false
	}
	return !s.payLifeGrantOffered(k, p, c, o, printed, av, cav)
}

// payLifeGrantApplies reports whether p's payment of cost settles {B} pips
// with life: p holds the grant (not merely the table: an opponent without
// K'rrik pays every {B} in mana) and the cost is one the kernel prices that
// way (no X and no snow pip). The planner's need (tapNeed) and the payment
// (payLifeGrantPay) read this one predicate, so a need the planner took off
// the deficit is always one the payment settles with life.
func (s *State) payLifeGrantApplies(k *prep, p uint8, cost *kop.ManaCost) bool {
	return k.gen != nil && k.gen.anyPayLife && cost.Pips[kop.B] > 0 && cost.X == 0 && cost.Snow == 0 &&
		s.payLifeInsteadOfB(k, p)
}

// payLifeGrantCovers reports whether the grant's life settles what p's pool
// leaves of cost (bLifeTake): the payment spends the life after the pool pays
// the rest (payLifeGrantPay).
func (s *State) payLifeGrantCovers(k *prep, p uint8, cost *kop.ManaCost) bool {
	if !s.payLifeGrantApplies(k, p, cost) {
		return false
	}
	_, _, ok := bLifeTake(cost, &s.pl[p].pool, int(s.pl[p].life))
	return ok
}

// payLifeGrantOffered is gorge's hand-cast offer gate under the grant (the
// printed cost's plain {B} pips; the offer prices the cost before the pip
// announcement). The floating pool pays it with the grant's life (the pool
// gate: a {B} pip 2 life settles), or the sources and pool pay it in mana
// alone (the planner settles with no life alternative,
// pay.PaymentSearchEnv): a cost whose generic part only the untapped sources
// pay and whose {B} pips only life pays is not offered. A cost the
// conversion left alone (a hybrid or printed Phyrexian pip, an X) keeps the
// kernel's single read.
func (s *State) payLifeGrantOffered(k *prep, p uint8, c CardID, o Obj, printed kop.ManaCost, av, cav *avail) bool {
	if printed.Pips[kop.B] == 0 || printed.Tb != 0 || printed.Hyb[0] != 0 || printed.X != 0 || printed.Snow != 0 {
		return true
	}
	plain := s.adjustFit(k, av, p, c, o, printed, false)
	var pav avail
	s.availPool(p, &pav)
	pav.life = s.pl[p].life
	pav.payLife = true
	if pav.affords(&plain, pipMaskOf(&plain), 0) {
		return true
	}
	nl := *cav
	nl.payLife = false
	return nl.affords(&plain, pipMaskOf(&plain), 0)
}

// payLifeGrantPlain undoes payLifeGrantCost: a cost whose Phyrexian pips are
// all {B} (the grant's conversion; a table with the grant prints no other
// {B/P} pip beside a {B}) reads as plain {B} pips again, the cost gorge's
// planner prices before manaAsk converts the pips.
func payLifeGrantPlain(c kop.ManaCost) kop.ManaCost {
	if c.Hyb[0] != kop.HybTwobrid || c.Hyb[1] != kop.HybPhyrexian || c.Tb == 0 {
		return c
	}
	n := c.TbN()
	for i := 0; i < n; i++ {
		if c.TbCol(i) != kop.B {
			return c
		}
	}
	c.Pips[kop.B] += uint8(n)
	c.Tb = 0
	c.Hyb[0], c.Hyb[1] = 0, 0
	return c
}

// bLifeTake is gorge's resolver assignment for a PayLifeInsteadOf:B payment
// (pay.ResolveManaWith's bLifeOK arm, CR 601.2h) over a fixed mana supply:
// the {B} pips the {B} units fund keep their mana (the resolver prefers it,
// pip by pip in printed order), a {B} pip gives its unit back when the rest
// of the cost would strand (the resolver's backtrack: the pool's {B} unit
// pays the generic), and every {B} pip left is settled with 2 life. It
// returns the mana-funded pip count and the life the split spends; ok
// reports whether the whole cost is payable this way. The pool's units are
// single-colour and independent, so the rest's Hall bound is arithmetic: a
// strict pip needs its own colour's units, the generic any leftover.
func bLifeTake(cost *kop.ManaCost, pool *[kop.NumManaKinds]uint8, life int) (b, spent int, ok bool) {
	n := int(cost.Pips[kop.B])
	rest := int(cost.Generic)
	for c := 0; c < kop.NumManaKinds; c++ {
		if c == kop.B {
			continue
		}
		rest += int(cost.Pips[c])
		if int(pool[c]) < int(cost.Pips[c]) {
			return 0, 0, false // a strict pip the supply lacks: no split helps
		}
	}
	tot := 0
	for c := range pool {
		tot += int(pool[c])
	}
	b = min(n, int(pool[kop.B]))
	for {
		if 2*(n-b) <= life && rest <= tot-b {
			return b, 2 * (n - b), true
		}
		if b == 0 {
			return 0, 0, false
		}
		b--
	}
}

// payLifeGrantReduce is the cost the pool pays once the grant's life has
// settled the {B} pips the pool leaves (payLifeGrantPay): rc holds the reduced
// copy and the result is cost itself when the grant settles nothing. Every
// payer that taps through the planner (tapNeed prices the grant's discount for
// all of them) and then pays from the pool reads this, so the discount and
// the life are one unit.
func (s *State) payLifeGrantReduce(k *prep, p uint8, cost *kop.ManaCost, x int, rc *kop.ManaCost) *kop.ManaCost {
	if s.payLifeGrantPay(k, p, cost, x, rc) {
		return rc
	}
	return cost
}

// payLifeGrantPay settles a PayLifeInsteadOf:B payment's {B} pips the pool
// leaves unfunded (bLifeTake, gorge payCast's ResolveManaWith) with 2 life
// each into out, before the pool pays the mana-funded rest. It reports
// whether it paid and out carries the reduced cost.
func (s *State) payLifeGrantPay(k *prep, p uint8, cost *kop.ManaCost, x int, out *kop.ManaCost) bool {
	if !s.payLifeGrantApplies(k, p, cost) {
		return false
	}
	b, spent, ok := bLifeTake(cost, &s.pl[p].pool, int(s.pl[p].life))
	if !ok || spent == 0 {
		return false
	}
	s.loseLife(k, p, spent)
	*out = *cost
	out.Pips[kop.B] = uint8(b)
	return true
}

// windowPoolOrGrantPays is windowPoolPays, or (for a non-any-colour pool) the
// payer's PayLifeInsteadOf:B grant settling the {B} pips the pool lacks.
func (s *State) windowPoolOrGrantPays(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) bool {
	return s.windowPoolPays(p, cost, anyColor) || !anyColor && s.payLifeGrantCovers(k, p, cost)
}
