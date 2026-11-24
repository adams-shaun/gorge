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

// restrLifeIrr is the irreversible cost of a unit whose view is pc: base plus
// 3 a point of a restricted life unit's life (pay.ConsequenceCost), and the
// life its activation pays (0: none).
func restrLifeIrr(pc *pcard, base uint8) (irr, life uint8) {
	if !restrLifeUnit(pc) {
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
// only after every other unit (gorge's TierLastResort).
func restrLifeLast(pc *pcard) bool { return restrLifeUnit(pc) && !DebugRestrictedLifeFirst }

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
	if pipN > 0 && !b.removeWide(uint8(1)<<uint(pipCol), uint16(pipN)) {
		// the pip's mana may sit in a wider group (an any-colour unit, a
		// one-colour entry inside a multi-bit one): take the pips from the
		// first group that holds them
		ok := false
		for i := 0; i < b.n; i++ {
			if b.mask[i]&(uint8(1)<<uint(pipCol)) != 0 && b.cnt[i] >= uint16(pipN) &&
				b.removeWide(b.mask[i], uint16(pipN)) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
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

// payPlanFee pays a plan unit's activation fee (utype.fee) from the pool: the
// pips from their colour, then the generic from the mana the payment's own
// pips do not claim (takeFeeGeneric, CR 601.2h). claim is the plan's claimed
// mana (feeClaim); the fee's own pips leave it as they are paid. The generic
// takes any kind, so the fee is payable exactly when the pool holds that much
// mana after the pips; a fee the pool cannot pay leaves the pool untouched.
func (s *State) payPlanFee(k *prep, p uint8, ty *utype, claim *[kop.NumManaKinds]int, anyCol bool) bool {
	pl := &s.pl[p]
	tot := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		tot += int(pl.pool[c])
	}
	c := -1
	if ty.feeCol != 0 {
		c = feeColIdx(ty.feeCol)
		if c < 0 || int(pl.pool[c]) < int(ty.feeN) {
			return false
		}
		tot -= int(ty.feeN)
	}
	g := int(ty.fee)
	if g > tot {
		return false
	}
	if c >= 0 {
		pl.pool[c] -= ty.feeN
		claim[c] = max(claim[c]-int(ty.feeN), 0)
	}
	takeFeeGeneric(&pl.pool, claim, anyCol, g)
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
