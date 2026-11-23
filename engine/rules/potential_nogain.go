package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// manaFeeNetsNoMana reports a mana ability whose whole cost c is mana (no
// {T}, no counter, no sacrifice) and which makes no more mana than it spends:
// Energy Refractor's "{2}: Add one mana of any color". However often a payment
// activates it, it converts colour and never raises the total mana the payer
// can spend (CR 106.1, 605.3b). A dynamic Amount$, or a tap trigger that adds
// mana beside it (extra, CR 605.4a), is not provably net-nothing.
func manaFeeNetsNoMana(c costvocab.Cost, mp *effects.ManaParams, extra state.Mana) bool {
	want := costvocab.Cost{Generic: c.Generic, Colored: c.Colored}
	fee := c.Generic + c.Colored.Total()
	if fee <= 0 || !c.Equal(&want) {
		return false
	}
	amt := availableAmountOf(mp)
	if amt <= 0 {
		return false
	}
	made := amt
	if !mp.CountsAny {
		made = 0
		for _, n := range mp.Counts {
			made += n * amt
		}
	}
	return made <= fee && extra.Total() == 0
}

// costOutrunsSupply reports that the mana of the composed cost c cannot be
// funded by count: it needs more mana than the pool (poolMana) and every
// census source (each tapped once, at its largest production) can make. It is
// a proof only beside a census whose missed abilities all net no mana
// (paymentPlanCensus.gapGains false), because those never add to the supply.
// A cost whose mana count can fall after announcement (an {X}, a Phyrexian
// symbol payable with life, a convoke/improvise/delve credit) is never proven.
//
// extra is the census's gapSupply: what its counter-bounded gap abilities add.
func costOutrunsSupply(pe pay.Engine, p state.PlayerID, poolMana, extra int32, c Cost, credit bool) bool {
	if credit || c.X != 0 || c.XMin != 0 || len(c.Phyrexian)+len(c.HybridPhyrexian) != 0 {
		return false
	}
	supply := poolMana + extra
	for _, u := range pay.PaymentPlanQueryUnits(pe, p) {
		best := int32(0)
		for _, a := range pay.PaymentPlanQueryAlternatives(pe, u) {
			best = max(best, int32(a.Mana.Total()))
		}
		supply += best
	}
	return supply < c.CMC()
}

// manaTapAbilityNetsNoMore reports a mana ability whose whole cost c is {T}
// and which makes no more mana than best, the most a source of the census
// already makes from its covered abilities: Chromatic Lantern's granted
// "{T}: Add one mana of any color" on a Swamp that already taps for {B}. The
// source taps once (CR 106.1, 605.3b), so the ability swaps one mana for
// another and never raises the count the census prices. A dynamic Amount$, or
// a tap trigger that adds mana beside it (extra, CR 605.4a), is not provable.
func manaTapAbilityNetsNoMore(c costvocab.Cost, mp *effects.ManaParams, extra state.Mana, best int32) bool {
	want := costvocab.Cost{Tap: c.Tap}
	if !c.Tap || !c.Equal(&want) || best <= 0 {
		return false
	}
	amt := availableAmountOf(mp)
	if amt <= 0 {
		return false
	}
	made := amt
	if !mp.CountsAny {
		made = 0
		for _, n := range mp.Counts {
			made += n * amt
		}
	}
	return made <= best && extra.Total() == 0
}

// gapOnceNetSupply reports the most mana a census-missed ONCE-ONLY fee-bearing
// ability ma can add beside the covered census units: every activation spends
// its mana fee (feeTotal, the planner's relaxed-alternatives fee, hybrid pips
// included) out of the mana the payment can spend and makes its fixed
// production, so the activation nets at most production − fee whatever funds
// the fee (CR 106.1, 605.3b, 601.2h). The tap is exclusive (CR 104.3a: one
// activation per tap), so what the figure may add is only what exceeds best,
// the source's own best covered production costOutrunsSupply already counts --
// the source either taps the covered ability or the gap, never both. once is
// paymentPlanRelaxedAlternatives's once-only verdict (a {T}/{U} cost, a
// once-only activation limit, a self-sacrifice or self-return part). False
// when the gap is repeatable (no count bound), has no mana fee (nothing
// leaves the budget), makes a dynamic amount, or a tap trigger adds mana
// beside it (extra, CR 605.4a). A net of zero or less converts colour and
// never raises the supply, so a bound of 0 still holds.
func gapOnceNetSupply(mp *effects.ManaParams, extra state.Mana, feeTotal, best int32, once, taps bool) (int32, bool) {
	if !once || !taps || feeTotal <= 0 || extra.Total() != 0 {
		return 0, false
	}
	amt := availableAmountOf(mp)
	if amt <= 0 {
		return 0, false
	}
	made := amt
	if !mp.CountsAny {
		made = 0
		for _, n := range mp.Counts {
			made += n * amt
		}
	}
	return max(made-feeTotal-best, 0), true
}

// gapBoundedSupply is the census's count bound for one gap ability ma that is
// neither net-free nor a gain: the SubCounter bound if ma has one, else the
// once-only fee-bearing net (gapOnceNetSupply: a once-only gap's fixed
// production nets its production minus the fee each activation spends, and the
// {T} is exclusive, so only what exceeds the source's own covered production
// best extends the count bound, CR 106.1; Fire-Lit Thicket's "{R/G}, {T}: Add
// {R}{R}"). A once-only gap without {T} shares no tap with the covered ability
// and is not bounded here. False means the caller records a gain.
func gapBoundedSupply(pp potentialPricer, py pay.Engine, p state.PlayerID, id state.ObjID, ma *cards.SA, mp *effects.ManaParams, extra state.Mana, fee, best int32, once, taps bool) (int32, bool) {
	if n, ok := gapSubCounterSupply(pp, py, p, id, ma, extra); ok {
		return n, true
	}
	return gapOnceNetSupply(mp, extra, fee, best, once, taps)
}

// gapSubCounterSupply reports the most mana a census-missed ability ma of
// source id can make, when its Amount$ X is bound by the counters its announced
// SubCounter<X/Kind> cost can settle (Mercadian Bazaar's "{T}, Remove any
// number of storage counters: Add {R} for each", one counter making one {R}),
// and true when ma is that shape. The source taps once (CR 106.1, 605.3b), so
// the figure is an upper bound on what the activation adds to the supply
// costOutrunsSupply counts; a tap trigger adding mana beside it (extra, CR
// 605.4a) is not provable.
func gapSubCounterSupply(pp potentialPricer, py pay.Engine, p state.PlayerID, id state.ObjID, ma *cards.SA, extra state.Mana) (int32, bool) {
	if extra.Total() != 0 {
		return 0, false
	}
	bounded, ok := potentialSubCounterBounded(pp, py, p, id, ma)
	if !ok {
		return 0, false
	}
	mp := effects.ManaOf(bounded)
	amt := availableAmountOf(mp)
	if mp.CountsAny {
		return amt, amt > 0
	}
	made := int32(0)
	for _, n := range mp.Counts {
		made += n * amt
	}
	return made, true
}
