package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Spend-restricted mana from a source whose cost is more than {T}.
//
// A restricted {T}, Pay N life ability of a source with no mana ability of its
// own (Gnarlroot Trapper's Elf-only {G}, Untaidake's legendary-only {C}{C})
// is a last-resort planner unit: gorge plans it as TierLastResort with the
// life a consequence (pay.PaymentPlanShapeTierOf), spent only after every
// other unit, the life paid as the unit is used (CR 119.4, 601.2g), and its
// mana floats restricted (CR 106.6).

// DebugRestrictedLifeFirst is the mutation hook of the restricted last-resort
// tier (dirac/oracle TestRestrictedManaLastResortMutation): a restricted
// {T}, PayLife unit is spent as a normal one, before the other units, instead
// of after them as gorge's TierLastResort does.
var DebugRestrictedLifeFirst bool

// restrLifeAb: a is a restricted "{T}, Pay N life" mana ability.
func restrLifeAb(a *kop.Ability) bool { return kop.RestrictedLifeShape(a) }

// restrSacSelfAb: a is a restricted "{T}, Sacrifice this" mana ability, a
// sacrificing last-resort unit like a Treasure's (pcard.manaSac).
func restrSacSelfAb(a *kop.Ability) bool { return kop.RestrictedSacSelfShape(a) }

// restrLifeUnit: pc is the view of a restricted last-resort life ability.
func restrLifeUnit(pc *pcard) bool { return pc.restrLife != 0 }

// restrLifePair: pc is a pain source whose painful alternative is a
// spend-restricted {T}, Pay N life ability (prepPainMana; Great Hall of the
// Biblioplex, Voldaren Estate). The pair itself carries the restriction and
// the float: plan leaves it to painUnit, which admits it per payment, rather
// than to a restricted unit of its own (restrViewW).
func restrLifePair(pc *pcard) bool { return pc.restrLife != 0 && pc.painCol != 0 && !pc.painOnly }

// restrLifeIrr is the irreversible cost of a unit whose view is pc: base plus
// 3 a point of a restricted life unit's life (pay.ConsequenceCost), and the
// life its activation pays (0: none).
func restrLifeIrr(pc *pcard, base uint8) (irr, life uint8) {
	if !restrLifeUnit(pc) || restrLifePair(pc) {
		// a pain pair's normal ability is an ordinary unit: only its painful
		// alternative pays the life (painUnit)
		return base, 0
	}
	if DebugRestrictedLifeFirst {
		return base, pc.restrLife // ranked as a free unit: key 1 no longer holds it last
	}
	return base + 3*pc.restrLife, pc.restrLife
}

// payUnitPain pays the pain of the planned unit of type ty at sl: a
// restricted unit's paid life (CR 119.4), else a painful unit's damage.
func (s *State) payUnitPain(k *prep, sl Slot, p uint8, ty *utype) {
	if ty.life {
		s.loseLife(k, p, int(ty.pain))
		return
	}
	s.painDamage(k, sl, p, int(ty.pain))
}

// restrLifePayable: record rec's life cost (a restricted {T}, Pay N life
// unit) leaves p above 0 life. gorge's planner withholds a last-resort
// alternative whose life reaches the payer's (pay.PlanLastResortChoices,
// CR 119.4); a record with no life cost is payable.
func (s *State) restrLifePayable(k *prep, p uint8, rec *restrRec) bool {
	n := k.gen.rs.views[rec.view].restrLife
	return n == 0 || int(s.pl[SeatIx(p)].life) > int(n)
}

// restrLifeOn: some restricted unit pays life, so the planner's lethal guard
// reads the payer's life even on a board with no painful source.
func restrLifeOn(k *prep) bool { return k.gen != nil && k.gen.rs != nil && k.gen.rs.life }

// restrLifeLast: pc is a restricted life unit plan leaves to phase 1, spent
// only after every other unit (gorge's TierLastResort). A pain pair's normal
// ability is not one: it stays in the ordinary census and only the pair's
// painful alternative joins the last-resort phase (painUnit).
func restrLifeLast(pc *pcard) bool {
	return restrLifeUnit(pc) && !restrLifePair(pc) && !DebugRestrictedLifeFirst
}

// restrLifeCharge counts a restricted life unit's n life against t, the
// census of a payment the unit may fund: gorge's lethal guard sums the unit's
// life with the damage of the painful sources the same plan taps and cuts a
// plan that reaches the payer's life (pay.PaymentPlanShapeTierOf, CR 119.4,
// 120.3). used is the life the payment's earlier restricted units charged.
// It reports false when the unit leaves no life for the plan to spend.
func (s *State) restrLifeCharge(k *prep, p uint8, t *avail, n int, used *int) bool {
	if int(s.pl[SeatIx(p)].life)-*used-n < 1 {
		return false
	}
	*used += n
	if t.pain.on {
		t.pain.budget -= uint8(n) // painTightAt kept life-1 ahead of the units' draw
		return true
	}
	if k.painMana {
		s.painTightAt(k, p, t, int(s.pl[SeatIx(p)].life)-*used)
	}
	return true
}

// restrFeeCost is fee unit v's activation fee as a cost: restrFee generic
// beside restrFeeN pips of restrFeeCol (kop.RestrictedManaCostShape's one
// colour), zero mana when the unit carries no fee.
func restrFeeCost(v *pcard) kop.ManaCost {
	var m kop.ManaCost
	m.Generic = v.restrFee
	if v.restrFeeCol != 0 {
		for c := 0; c < 5; c++ {
			if v.restrFeeCol&(1<<uint(c)) != 0 {
				m.Pips[c] = v.restrFeeN
			}
		}
	}
	return m
}

// restrFeeEmpty reports whether fee unit v's view carries no fee.
func restrFeeEmpty(v *pcard) bool { return v.restrFee == 0 && v.restrFeeCol == 0 }

// restrFeeCharge charges the fee unit's fee (restrFeeCost's) from a, the
// mana counted beside the unit (gorge's cast window pays the fee from the
// mana the window has already accumulated before the production joins,
// engine/rules cast_window.go castWindowReachableAfter, CR 605.3b), and
// reports whether it was: a fee the mana cannot pay leaves the unit out of
// the bound. The payment the fee is charged against is priced after the
// walk, by its own afford check over the mana the unit's production joins --
// the charge only takes the fee's mana out (the pips first, they are
// colour-locked; the generic from the groups the cost's pips do not need,
// then the rest, CR 601.2h). The search over the generic's assignment is
// bounded by a node budget and fails closed (gorge's own reach search is
// node-bounded the same way).
func (a *avail) restrFeeCharge(fee kop.ManaCost, cost *kop.ManaCost, extra int) bool {
	if fee.MV() == 0 {
		return true
	}
	if !a.affords(&fee, pipMaskOf(&fee), 0) {
		return false // the fee is not payable from the mana counted at all
	}
	b := *a
	var pipCol, pipN int
	for c := 0; c < 5; c++ {
		if fee.Pips[c] != 0 {
			pipCol, pipN = c, int(fee.Pips[c])
		}
	}
	if pipN > 0 && !b.restrFeePips(pipCol, pipN) {
		return false
	}
	if fee.Generic == 0 {
		*a = b
		return true
	}
	nodes := 0
	if b.restrFeeSplit(int(fee.Generic), 0, cost, &nodes) {
		*a = b
		return true
	}
	return false
}

// restrFeePips takes n pips of colour col from a's groups: a single group
// holding them all first (removeWide's one-colour bookkeeping), else the
// pips split over the groups that can make col, an exact-colour group before
// a wider one (Castle Garenbrig's {2}{G}{G} fee from one Forest and one
// any-colour source). A source that makes all its mana one colour has its
// entry reduced with the units taken, so a later colour assignment cannot
// count the spent units again.
func (a *avail) restrFeePips(col, n int) bool {
	bit := uint8(1) << uint(col)
	if a.removeWide(bit, uint16(n)) {
		return true
	}
	left := n
	for pass := 0; pass < 2 && left > 0; pass++ {
		for i := 0; i < a.n && left > 0; i++ {
			if a.mask[i]&bit == 0 || a.cnt[i] == 0 {
				continue
			}
			if (a.mask[i] == bit) != (pass == 0) {
				continue // exact-colour groups first, then wider ones
			}
			take := min(left, int(a.cnt[i]))
			if !a.takeWide(a.mask[i], uint16(take)) {
				continue
			}
			a.takeOneColour(a.mask[i], uint8(take))
			left -= take
		}
	}
	return left == 0
}

// takeOneColour reduces a one-colour entry matching mask by n, dropping it
// when spent: restrFeePips' partial removal, where removeWide's exact-amount
// dropOneColour would leave the entry counting spent units.
func (a *avail) takeOneColour(mask, n uint8) {
	for i := uint8(0); i < a.nOne; i++ {
		m := a.oneMask[i]
		if m == 0 {
			m = uint8(kop.ColAny)
		}
		if m != mask || a.one[i] < n {
			continue
		}
		a.one[i] -= n
		if a.one[i] == 0 {
			a.nOne--
			a.one[i] = a.one[a.nOne]
			a.oneMask[i] = a.oneMask[a.nOne]
			a.oneAltMask[i] = a.oneAltMask[a.nOne]
			a.oneAltN[i] = a.oneAltN[a.nOne]
			a.oneCol[i] = a.oneCol[a.nOne]
		}
		return
	}
}

// restrFeeFundAvail builds a, the mana a cast's restricted fee may be paid
// from beside the payment's ordinary bound: the floating pool and every other
// restricted-fee source's plain census ability (Castle Garenbrig's
// {T}: Add {G}). Gorge's cast window funds a fee activation from mana an
// earlier same-window activation made (cast_window.go castWindowReachable), so
// another restricted-fee source's plain ability pays the fee (CR 605.3b) even
// where the census's relaxed paid list is cleared. An ordinary source's mana
// is the payment's own bound or the census's funding, never this extra pool.
// deferSkip drops the Taps-deferred sources the census leaves out
// (availForMode).
func (s *State) restrFeeFundAvail(k *prep, p uint8, a *avail, deferSkip bool) {
	pl := &s.pl[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		if pl.pool[c] > 0 {
			a.addUnits(1<<uint(c), pl.pool[c])
		}
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if k.gen == nil || k.gen.flags[s.cardAt(sl)]&gfRestrSrc == 0 {
			continue
		}
		if deferSkip && s.planDeferred(k, p, sl) && !s.blgIntrinsicUnit(k, sl) {
			continue
		}
		own := s.manaPCGen(k, sl)
		if own.manaAb < 0 || own.restrFloat || !restrFeeEmpty(own) || !s.canTapForManaPC(k, sl, own) {
			continue
		}
		a.addUnits(s.srcMaskAt(own, sl), own.manaN)
	}
}

// restrFeeFundPays reports whether fee unit own (record kind kind) of the
// source at sl has its fee fc payable from fund (restrFeeFundAvail); on
// success it takes the fee out of fund, so the next fee unit cannot spend the
// same mana. A widened source taps once, so its own census unit is taken out
// first, as restrSrcAddUnit's rkWiden arm does.
func (s *State) restrFeeFundPays(own, v *pcard, kind uint8, sl Slot, fund *avail, fc kop.ManaCost, cost *kop.ManaCost, extra int) bool {
	b := *fund
	if kind == rkWiden {
		if !b.removeWide(s.srcMaskAt(own, sl), uint16(own.manaN)) && restrComboWide(v) {
			return false // its census {C} already paid an earlier fee: its one tap is spent
		}
	}
	if !b.restrFeeCharge(fc, cost, extra) {
		return false
	}
	*fund = b
	return true
}

// restrFeeCastFunded reports whether a cast's fee is payable from another
// restricted-fee source's plain census ability: gorge's cast window funds a
// fee activation from an earlier same-window activation (cast_window.go
// castWindowReachable), so such a fee is not the census's relaxed paid list's
// to cover. feeFund is built on first use. The unit's charge still fails
// closed when that mana is short.
func (s *State) restrFeeCastFunded(k *prep, p uint8, d payDesc, own, v *pcard, kind uint8, sl Slot, feeFund **avail, deferSkip bool, fc kop.ManaCost, cost *kop.ManaCost, extra int) bool {
	if d.class != pdSpell {
		return false
	}
	if *feeFund == nil {
		*feeFund = new(avail)
		s.restrFeeFundAvail(k, p, *feeFund, deferSkip)
	}
	return s.restrFeeFundPays(own, v, kind, sl, *feeFund, fc, cost, extra)
}

// restrFeeChargePlain is restrFeeCharge from plain, the unrestricted part of
// a: a restricted unit's mana pays no mana ability's fee (CR 106.6). The mana
// plain's charge takes leaves a too, group by group; a fee plain cannot pay
// leaves both untouched.
func (a *avail) restrFeeChargePlain(plain *avail, fee kop.ManaCost, cost *kop.ManaCost, extra int) bool {
	b := *plain
	if !b.restrFeeCharge(fee, cost, extra) {
		return false
	}
	c := *a
	for i := 0; i < plain.n && i < b.n; i++ {
		if n := plain.cnt[i] - b.cnt[i]; n != 0 && !c.removeWide(plain.mask[i], n) {
			return false
		}
	}
	*a, *plain = c, b
	return true
}

// payPlanFee pays a plan unit's activation fee (utype.fee) from the pool: the
// pips from their colour, then the generic from the mana the payment's own
// pips do not claim (takeFeeGeneric, CR 601.2h). claim is the plan's claimed
// mana (feeClaim); the fee's own pips leave it as they are paid. The generic
// takes any kind, so the fee is payable exactly when the pool holds that much
// mana after the pips; a fee the pool cannot pay leaves the pool untouched.
// lent is the restricted mana the hybrid witness lent into the pool for the
// spell's own cost: a fee unit's activation is a different payment, so the
// lent units stay out of the fee's supply (CR 106.6) and the fee is paid from
// the pool around them.
func (s *State) payPlanFee(k *prep, p uint8, ty *utype, claim *[kop.NumManaKinds]int, anyCol bool, lent [kop.NumManaKinds]uint8) bool {
	pl := &s.pl[p]
	var avail [kop.NumManaKinds]uint8
	tot := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		if int(pl.pool[c]) < int(lent[c]) {
			return false // the fee cannot spend the lent units; keep the pool untouched
		}
		avail[c] = pl.pool[c] - lent[c]
		tot += int(avail[c])
	}
	c := -1
	if ty.feeCol != 0 {
		c = feeColIdx(ty.feeCol)
		if c < 0 || int(avail[c]) < int(ty.feeN) {
			return false
		}
		tot -= int(ty.feeN)
	}
	g := int(ty.fee)
	if g > tot {
		return false
	}
	if c >= 0 {
		avail[c] -= ty.feeN
		claim[c] = max(claim[c]-int(ty.feeN), 0)
	}
	takeFeeGeneric(&avail, claim, anyCol, g)
	for c := 0; c < kop.NumManaKinds; c++ {
		pl.pool[c] = avail[c] + lent[c]
	}
	return true
}

// feeClaim is the mana the plan's payments claim by colour: the cost's pips
// and the pips of every chosen fee alternative. A fee's generic leaves it to
// them while the pool holds other mana (takeFeeGeneric).
func (pk *planner) feeClaim() (claim [kop.NumManaKinds]int) {
	if !pk.anyFee {
		return
	}
	pips, _, _ := pk.feeOwed()
	for c := range claim {
		claim[c] = int(pk.cpips[c]) + pips[c]
	}
	return
}

// feeRem adds the chosen fee alternatives' pips to the colours the units are
// to make (valid covers them beside the cost's).
func (pk *planner) feeRem(rem *[kop.NumManaKinds]int, remAny *int) {
	if !pk.anyFee {
		return
	}
	pips, _, _ := pk.feeOwed()
	for c, n := range pips {
		rem[c] += n
		*remAny += n
	}
}

// takeFeeGeneric takes g generic mana of an activation fee from pool (CR
// 601.2h): first the mana the claim does not hold back, then the rest, each in
// kind index order. The offer's restrFeeSplit prices the same rule (the
// groups the cost's pips do not use first); taking in kind order alone spent
// the colour the spell's own pips needed (the {U}{U} of a {7}{U}{U} creature
// paid a Castle's generic). anyCol: the coloured claim is one fungible
// category (spend as any colour), held from the first coloured kinds the pool
// has.
func takeFeeGeneric(pool *[kop.NumManaKinds]uint8, claim *[kop.NumManaKinds]int, anyCol bool, g int) {
	hold := *claim
	if anyCol {
		n := 0
		for c := 0; c < 5; c++ {
			n += hold[c]
			hold[c] = 0
		}
		for c := 0; c < 5 && n > 0; c++ {
			hold[c] = min(n, int(pool[c]))
			n -= hold[c]
		}
	}
	for pass := 0; pass < 2 && g > 0; pass++ {
		for c := 0; c < kop.NumManaKinds && g > 0; c++ {
			have := int(pool[c])
			if pass == 0 {
				have = max(have-hold[c], 0)
			}
			d := min(g, have)
			pool[c] -= uint8(d)
			g -= d
		}
	}
}

// restrFeeSplit takes g generic mana for the fee from a's groups (the
// payer's colour assignment, CR 601.2h) and reports whether some assignment
// takes all of g. cost, when given, orders the groups: the ones its pips do
// not draw from are tried first, so the fee's generic does not take mana the
// payment's colour-locked pips need when another group can fund it. nodes
// bounds the search.
func (a *avail) restrFeeSplit(g, i int, cost *kop.ManaCost, nodes *int) bool {
	if *nodes > 512 {
		return false
	}
	*nodes++
	if g == 0 {
		return true
	}
	var pm uint8
	if cost != nil {
		pm = pipMaskOf(cost)
	}
	var ord [32]int
	on := 0
	for pass := 0; pass < 2 && on < len(ord); pass++ {
		for j := 0; j < a.n && on < len(ord); j++ {
			if j < i || a.mask[j]&0x3f == 0 || a.cnt[j] == 0 {
				continue // no mana of a pay kind in the group
			}
			if (a.mask[j]&pm != 0) == (pass == 0) {
				continue
			}
			ord[on] = j
			on++
		}
	}
	for x := 0; x < on; x++ {
		j := ord[x]
		// Try in place and restore: a copy whose address the recursive call
		// takes would move to the heap (TestNoKernelLocalEscapes).
		saved := *a
		for k := min(g, int(a.cnt[j])); k > 0; k-- {
			if a.removeWide(saved.mask[j], uint16(k)) && a.restrFeeSplit(g-k, j+1, cost, nodes) {
				return true
			}
			*a = saved
		}
	}
	return false
}
