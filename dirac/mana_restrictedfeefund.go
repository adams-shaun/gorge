package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The restricted fee gap's funding: gorge's payment census
// (engine/rules/potential_plan.go paymentPlanCensusOf) prices every
// activatable mana ability of p's sources, and a restricted fee ability is
// never among them (the census's own fee walk skips a RestrictValid$
// ability, engine/rules/paid_mana_census.go), so it is a gap. The census's
// relaxed proof books each fee gap it can relax as a `paid` entry carrying
// its fee -- at most relaxedPaidLimit over the whole board, zone order (the
// filters' pip fees are entries of the same list) -- and a later one clears
// the list (relaxable false). The prefix search then pays a fee gap's fee
// from the floating pool, or by tapping covered sources, at most
// census.fees() of them (engine/rules/potential_script.go
// potentialPrefixScript's funding). So a restricted fee unit prices for a
// payment only when the payer's floating pool pays its fee, or the census's
// funding covers what the pool leaves (restrSrcAvailD's fee pass,
// mana_restrictedcost.go's charge bound): with the paid list cleared the
// search funds nothing and the fee unit leaves the census standing.

// restrFeeFund is the census's funding (census.fees) for payer p, and
// whether the census's paid list is cleared, read over p's zone-ordered
// activatable fee gaps: the board's filters (scriptFilt) and the restricted
// fee abilities (prepRestrSrc's fee views). A repeatable fee gap (a
// repeatable fee-only filter) makes the funding unbounded instead
// (paymentPlanCensusOf's out.fund books fee x potentialScriptNodes for one,
// and clears the paid list). A hybrid fee is unrelaxable the same way, but
// its once-only funding (census.fund) survives the clear.
func (s *State) restrFeeFund(k *prep, p uint8) (fund, wiped int) {
	fm := s.scriptFilt(k, p)
	if s.filtRepeatIn(k, fm) {
		return potentialScriptNodes, 1
	}
	hybFund := 0
	for b := fm; b.any(); b = b.rest() {
		if _, _, h := s.filtHyb(k, b.first()); h {
			hybFund += s.filtFee(k, b.first()) // census.fund: a hybrid fee's funding survives the clear
		}
	}
	wiped = hybFund
	if s.storeGapLive(k, p) || s.reflAmtLive(k, p) ||
		(k.tapMana && (s.tapXTwoLive(k, p) || s.tapXSacAnyLive(k, p))) {
		wiped = 1
	}
	n := 0
	if wiped == 0 {
		for b := fm; b.any(); b = b.rest() {
			sl := b.first()
			if _, _, h := s.filtHyb(k, sl); h {
				continue // its fee is census.fund's, counted above
			}
			n++
			fund += s.filtFee(k, sl)
		}
		for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
			sl := m.first()
			if s.perms[sl].flags&pfTapped != 0 {
				continue // a tapped source's abilities are none of the census's (CR 602.2b)
			}
			c := s.cardAt(sl)
			if k.gen == nil || k.gen.flags[c]&gfRestrSrc == 0 {
				continue
			}
			for i := range k.gen.rs.recs {
				rec := &k.gen.rs.recs[i]
				if rec.card != c {
					continue
				}
				v := &k.gen.rs.views[rec.view]
				if v.restrPair != 0 {
					v = &k.gen.rs.views[v.restrPair-1] // a free view's fee alternative is a gap of its own (mana_restrictedfeepair.go)
				}
				if restrFeeEmpty(v) {
					continue
				}
				n++
				fund += int(v.restrFee) + int(v.restrFeeN)
			}
		}
		if n > relaxedPaidLimit {
			wiped, fund = 1, hybFund // the fourth paid entry clears the list (relaxable false)
		}
	}
	return fund, wiped
}

// restrFeePool is the floating pool's mana: the part of the fees' mana value
// the prefix search's funding need not tap for.
func (s *State) restrFeePool(p uint8) int {
	pl := &s.pl[SeatIx(p)]
	tot := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		tot += int(pl.pool[c])
	}
	return tot
}
