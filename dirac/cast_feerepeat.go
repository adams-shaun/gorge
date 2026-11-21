package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// castGateFeeRepeat is the hand-cast offer gate's read of the live repeatable
// fee-only mana abilities (kop.FeeOnlyManaAb, Signpost Scarecrow's "{2}: Add
// one mana of any color", pool tail-45-c004 decision 777). gorge's payment
// planner plans their activations while paying (CR 601.2i): each one converts
// mana of the pool the plan taps into one unit of its production at its fee
// (CR 605.3a), so the printed census -- which counts a source with a tap or
// sacrifice ability alone -- has no unit for a source that pays to make, and
// refuses a cast the plan funds (Historian of Zhalfir's {2}{U}{U} beside one
// untapped Island, three Forests, a Sandstone Needle and the Scarecrow: the
// Island's {U} fills one pip and the Island's and two Forests' mana the fee,
// whose one mana of any colour fills the second).
//
// The gate prices the planner's conversions over cav, that census: k
// activations of a filter put k units of its production beside cav and charge
// their fees on the cost's generic part, and the cast is offered when some
// counts leave it affordable (gorge pay/solve's composition, whose conversion
// option is the same arithmetic at the pip it is asked for). A conversion
// makes no more than its fee spends (FeeOnlyManaAb), so counts beyond the
// cost's mana value only lose mana: the loop stops there. The fee-side census
// is cav (castGrantAvail's: a restricted grant the cast cannot spend is no
// unit of it, CR 106.6; gorge plans the fee from the payment's AvailableFor
// pool too), and the filters are scriptFilt's, whose repeatable members stand
// only when the potential bound pays their fee (filtRepeatPotential, gorge's
// census-gap condition, paymentPlanCensusOf).
//
// Not in the scripted any-colour and gaps walks (scriptOffers): their
// censuses price their own lines, and a play they list is gorge's potential
// play, which its prefix search prices, not the planner. The read is the
// oracle surfaces': a base surface has no kernel mirror of the planner route
// to model.
func (s *State) castGateFeeRepeat(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, cav *avail) bool {
	if k.gen == nil || !k.gen.anyFiltRepeat || s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) == 0 ||
		s.g.availX&(availAny|availGaps) != 0 || !s.filtRepeatIn(k, s.scriptFilt(k, p)) {
		return false
	}
	var fees, masks [paidGapMax]uint8
	n := 0
	for b := s.scriptFilt(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if !s.filtRepeat(k, sl) || !s.filtUsable(k, sl) {
			continue
		}
		ab := s.filtAb(k, sl)
		if ab.AFlags&kop.AfManaDynamic != 0 || ab.ManaEach || ab.ManaN > 1 ||
			ab.AFlags&^kop.AfManaChoice != 0 {
			continue // a production the plan does not price one conversion of
		}
		f := uint8(s.filtRepeatFee(k, sl))
		if f == 0 || int(f) > cav.total {
			continue // no fee, or one the census cannot pay
		}
		if n < paidGapMax {
			fees[n], masks[n] = f, uint8(ab.ManaColors)
			n++
			continue
		}
		// gorge's relaxed proof prices subsets of at most paidGapMax paid
		// abilities (relaxedPaidLimit); the gate keeps that line at the
		// cheapest fees: a dearer conversion fills a pip only the dearer
		// ones cannot, a shape the census's over-bound already admits.
		// (a dearer filter beyond the cap is dropped: the census over-bound admits it)
		for i := 0; i < paidGapMax; i++ {
			if fees[i] > f {
				fees[i], masks[i], f = f, uint8(ab.ManaColors), fees[i]
			}
		}
	}
	if n == 0 {
		return false
	}
	maxK := int(cost.MV())
	var ks [paidGapMax]int
	var walk func(i, left int) bool
	walk = func(i, left int) bool {
		if i == n {
			t, gen := 0, int(cost.Generic)
			for j := 0; j < n; j++ {
				t += ks[j] * int(fees[j])
				gen += ks[j] * int(fees[j])
			}
			if t > cav.total || gen > 255 {
				return false // the fees outrun the census mana (CR 106.6's spendable pool)
			}
			a := *cav
			for j := 0; j < n; j++ {
				if ks[j] != 0 {
					a.addUnits(masks[j], uint8(ks[j]))
				}
			}
			cx := *cost
			cx.Generic = uint8(gen)
			return a.affords(&cx, pipMaskOf(&cx), 0)
		}
		for j := 0; j <= left; j++ {
			ks[i] = j
			if walk(i+1, left-j) {
				return true
			}
		}
		ks[i] = 0
		return false
	}
	return walk(0, maxK)
}
