package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// paidGapMax mirrors gorge's relaxedPaidLimit: a relaxed payment proof
// enumerates subsets of at most this many paid mana abilities its planner
// census misses, and a board with more has no proof against an X (the
// over-bound stands).
const paidGapMax = 3

// gapNote records the paid mana ability j of slot sl (fee generic mana, a
// filter's {1}) as one of the walk's paid gaps and reports whether the probe
// leaves it out. The index is the encounter order, which the fixpoint's first
// pass fixes the same for every run, so gapSkip's bit i names the same
// ability in each subset run (xBoundGapSubsets).
func (d *payDesc) gapNote(sl Slot, j, fee int, never bool) bool {
	id := uint16(sl)<<4 | uint16(j)
	for i := 0; i < int(d.gapN) && i < paidGapMax; i++ {
		if d.gapID[i] == id {
			return d.gapSkip>>i&1 != 0
		}
	}
	i := int(d.gapN)
	if i < paidGapMax {
		d.gapID[i], d.gapFee[i] = id, uint8(min(fee, 255))
		if never {
			d.gapNever |= 1 << i
		}
	}
	if d.gapN < 255 {
		d.gapN++
	}
	return i < paidGapMax && d.gapSkip>>i&1 != 0
}

// paidGapProbe is availPotentialFor's read of a mana ability's paid fee: paid
// reports a fee-bearing ability while the per-tap census runs, skip that the
// walk leaves it out, repeat that its cost has no tap, sacrifice-self or
// once-only limit. The census (units) is the planner-shaped bound: gorge's
// relaxed proof charges a paid ability's fee as generic on top of the play's
// cost (paymentPlanRelaxProof), while the pool mana that covers the fee here
// is already a recorded unit, so admitting it would double-count (White Lotus
// Hideout's {1},{T} filter). The over-bound pool beside the census keeps the
// paid abilities (boundedAny false skips them here); the X-bound sweep's
// probe (d.gapTrack) names each one so the subsets can be priced. repeat is
// only the flags read: it marks d.gapRepeats (mana_restrictedsource.go) at
// the ability's admission, past availPotentialFor's fundability gate, because
// gorge's once gate reads the census's admitted abilities alone — a gap whose
// fee nothing covers never reaches it (Nomadic Elf's {1}{G} beside white
// mana, engine/rules/potential_plan.go's paymentPlanCensusOf).
func (d *payDesc) paidGapProbe(census, boundedAny bool, sl Slot, j int, ab *kop.Ability) (paid, skip, repeat bool) {
	fee := &ab.Cost.Mana
	paid = census && fee.MV() != 0
	switch {
	case !paid:
	case !boundedAny:
		skip = true
	case d != nil && d.gapTrack:
		skip = d.gapNote(sl, j, fee.MV(), kop.HybridFilterManaAb(ab))
		repeat = ab.Cost.Flags&(kop.CostTap|kop.CostSacSelf|kop.CostOncePerTurn|kop.CostOncePerGame) == 0
	}
	return paid, skip, repeat
}

// gapSubsets prices an X-bound target's announcement the way gorge's relaxed
// payment proof does when the planner's census misses paid abilities (Golgari
// Signet's {1},{T}: Add {B}{G}): for each subset of those abilities, the
// walk's pool with only that subset's productions beside the cost with the
// subset's fees charged as generic on top (CR 605.3b, 601.2g). An ability
// whose fee nothing else funds adds nothing, so a filter beside one other
// source pays no more than that source does.
type gapSubsets struct {
	n   int // the subsets priced; 0 when the board has no paid gap or more than paidGapMax
	sub [1 << paidGapMax]gapSubset
}

type gapSubset struct {
	av  avail
	fee int
}

// xBoundGapSubsets fills gs for the payment d (a copy: the probe's fields are
// its own).
func (s *State) xBoundGapSubsets(k *prep, p uint8, d payDesc, gs *gapSubsets) {
	var pu [maxPotUnits]potUnit
	var av avail
	d.gapTrack, d.gapSkip, d.gapN, d.gapRepeats, d.gapNever = true, 0, 0, false, 0
	s.availPotentialFor(k, p, &av, &d, true, &pu)
	n := int(d.gapN)
	if n == 0 || n > paidGapMax || d.gapRepeats {
		// a repeatable gap (Crossroads Candleguide's "{2}: Add one mana of any
		// color") makes gorge's census non-relaxable: its relaxed proof never
		// runs, so no subset pricing refuses the announcement and the
		// over-bound stands (paymentPlanCensusOf's once gate, CR 605.3b)
		return
	}
	fees := d.gapFee
	for m := 0; m < 1<<n; m++ {
		if never := int(d.gapNever); m&never != 0 {
			// a hybrid-fee filter funds nothing in gorge's prefix search (its
			// fee is paid only from mana already floating), so a subset
			// holding it is priced as the one without it, fee uncharged
			gs.sub[m] = gs.sub[m&^never]
			continue
		}
		dm := d
		dm.gapSkip, dm.gapN = ^uint8(m), 0
		var sub avail
		var pm [maxPotUnits]potUnit
		s.availPotentialFor(k, p, &sub, &dm, true, &pm)
		fee := 0
		for i := 0; i < n; i++ {
			if m>>i&1 != 0 {
				fee += int(fees[i])
			}
		}
		gs.sub[m] = gapSubset{av: sub, fee: fee}
	}
	gs.n = 1 << n
}

// affords: some subset pays cost plus its fees.
func (gs *gapSubsets) affords(cx *kop.ManaCost) bool {
	for m := 0; m < gs.n; m++ {
		c := *cx
		c.Generic = uint8(min(int(c.Generic)+gs.sub[m].fee, 255))
		if gs.sub[m].av.affords(&c, pipMaskOf(&c), 0) {
			return true
		}
	}
	return false
}
