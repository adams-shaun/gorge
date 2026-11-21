package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

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

// gapProd is a paid gap's production as the lone-gap cut reads it (gorge
// relaxedPaid.plain, relaxedCutPlain): plain is a fixed or one-colour-choice
// production of one mana per colour, no extra, restriction, rider, dynamic or
// colourless part. each: every colour of mask at once (Golgari Signet's {B}{G});
// otherwise one mana of one colour of mask (Unknown Shores' any colour).
type gapProd struct {
	mask  uint8
	each  bool
	plain bool
}

// gapProdOf reads ab's production for the cut. tapExtra is the ability's own
// TapsForMana extra (paidGapTapExtra), which adds mana the cut does not model.
func gapProdOf(ab *kop.Ability, tapExtra bool) gapProd {
	c := ab.ManaColors
	plain := !tapExtra && ab.AFlags&^kop.AfManaChoice == 0 && ab.ManaRestrict == 0 && ab.ManaRestrictAb == 0 &&
		ab.ManaN <= 1 && c != 0 && c&^kop.ColAny == 0 && ab.SplitGate == 0 && ab.ManaV == (kop.ValueRef{}) &&
		ab.ManaRider == kop.ManaRiderNone && ab.ManaHasteValid == 0 && !ab.ManaGrave
	return gapProd{mask: uint8(c), each: ab.ManaEach, plain: plain}
}

// gapProdNote records the production of the gap named by slot sl's ability j
// (gapNote's id), once the gap is booked.
func (d *payDesc) gapProdNote(sl Slot, j int, pr gapProd) {
	id := uint16(sl)<<4 | uint16(j)
	for i := 0; i < int(d.gapN) && i < paidGapMax; i++ {
		if d.gapID[i] == id {
			d.gapProd[i] = pr
			return
		}
	}
}

// cutVecs calls f with each way the gap's production can pay a cost's pips:
// the whole mask at once, or one colour of the mask.
func (pr gapProd) cutVecs(f func(v [kop.NumManaKinds]uint8) bool) bool {
	var v [kop.NumManaKinds]uint8
	if pr.each {
		for c := 0; c < 5; c++ {
			if pr.mask&(1<<c) != 0 {
				v[c] = 1
			}
		}
		return f(v)
	}
	for c := 0; c < 5; c++ {
		if pr.mask&(1<<c) != 0 {
			v = [kop.NumManaKinds]uint8{}
			v[c] = 1
			if f(v) {
				return true
			}
		}
	}
	return false
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
	case d != nil && d.gapSkipPaid:
		skip = true
	case !boundedAny:
		skip = true
	case d != nil && d.gapTrack:
		skip = d.gapNote(sl, j, fee.MV(), kop.HybridFilterManaAb(ab))
		repeat = ab.Cost.Flags&(kop.CostTap|kop.CostSacSelf|kop.CostOncePerTurn|kop.CostOncePerGame) == 0
	}
	return paid, skip, repeat
}

// paidGapBooked is availPotentialFor's bookkeeping for an admitted paid gap:
// the census was already marked incomplete (cen.gap), so the gap is named for
// the subset pricing (gapNote) and gapGain set exactly as gorge's
// paymentPlanCensus.gapGains sets it for a missed ability
// (rules/potential_nogain.go manaFeeNetsNoMana): a gap that is not a plain
// mana fee, that makes more mana than it spends, or that brings a tap extra
// raises the mana a payment can spend, so the covered census alone is no
// honest supply bound for the X-bound sweep's count arm (CR 106.1, 605.3b).
// tapExtra is the ability's own TapsForMana extra (paidGapTapExtra), never
// the table-wide "some card has the static" flag.
func paidGapBooked(d *payDesc, sl Slot, j int, ab *kop.Ability, tapExtra bool) {
	if d == nil {
		return
	}
	d.gapNote(sl, j, int(ab.Cost.Mana.MV()), kop.HybridFilterManaAb(ab))
	d.gapProdNote(sl, j, gapProdOf(ab, tapExtra))
	if tapExtra || !paidGapNetsNoMana(ab) {
		d.gapGain = true
	}
}

// bookPaidGap is availPotentialFor's bookkeeping for an admitted paid gap. Its
// default read leaves the ability a census gap: the sweep's per-tap view cannot
// price that filter, so its "insufficient" is no proof against an X the relaxed
// census pays (Unknown Shores' {1},{T} beside Plains and Radiant Grove for
// Killing Glare, X=1; CR 605.3b, 601.2g), and the over-bound pool takes it.
// The subset pricing (d.gapPrice, xBoundGapSubsets) instead keeps the
// production as a census unit and charges its fee as generic on top, so its
// record stays exact. An admitted repeatable gap is one of gorge's census's
// non-once abilities: its census is not relaxable, so no subset pricing refuses
// the announcement and the over-bound stands (paymentPlanCensusOf's once gate,
// CR 605.3b). A gap the fundability gate skipped is out of gorge's census
// entirely and never reaches here.
func (s *State) bookPaidGap(k *prep, d *payDesc, sl Slot, j int, ab *kop.Ability, cen *potCensus, repeat bool) {
	if d == nil || !d.gapPrice {
		cen.gap()
		// the sweep's gap bookkeeping: the gap named, its fee recorded, and
		// gapGain when the ability can raise the mana a payment spends
		paidGapBooked(d, sl, j, ab, s.paidGapTapExtra(k, sl, ab))
	}
	if repeat && d != nil {
		d.gapRepeats = true
	}
}

// paidGapNetsNoMana mirrors manaFeeNetsNoMana for a paid gap: the whole cost
// is plain mana (no {T}, {X}, hybrid, snow or other part), it spends a
// positive fee, and its production is statically priced no higher than that
// fee. A dynamic amount has no static price (gorge's availableAmountOf
// returns 0, so the proof refuses), while an each-colour production counts
// every colour it makes and a choice counts its one colour once.
func paidGapNetsNoMana(ab *kop.Ability) bool {
	if !paidGapPlainManaCost(&ab.Cost) {
		return false
	}
	fee := ab.Cost.Mana.MV()
	if fee <= 0 {
		return false
	}
	made, ok := paidGapMakes(ab)
	return ok && made <= fee
}

// paidGapPlainManaCost matches costvocab.Cost.Equal against {Generic,
// Colored}: an activation limit or sorcery timing is not a cost part, every
// other flag or table byte is.
func paidGapPlainManaCost(c *kop.Cost) bool {
	if c.Flags&^(kop.CostOncePerTurn|kop.CostOncePerGame|kop.CostSorcery) != 0 {
		return false
	}
	if c.Host != 0 || c.N != 0 || c.Ctr != 0 || c.Filter != 0 || c.PutLibSelf || c.ExileHand ||
		c.Exert || c.DiscardHand || c.DiscardRandom || c.XMin != 0 || c.ExileBf || c.ExileGSelf ||
		c.Mill != 0 || c.RevealSelf {
		return false
	}
	m := c.Mana
	return m.X == 0 && m.XPip == 0 && m.Hyb == ([4]kop.ColorMask{}) && m.Tb == 0 && m.Snow == 0
}

// paidGapMakes is the mana the ability produces at its static price: an
// each-colour production makes ManaN of every named colour (gorge's
// ProducedCounts sum), while every other shape is priced once at ManaN (a
// Combo/Any production is CountsAny, so gorge reads the amount once). A
// dynamic amount has no static price.
func paidGapMakes(ab *kop.Ability) (int, bool) {
	if ab.AFlags&kop.AfManaDynamic != 0 {
		return 0, false
	}
	if ab.ManaEach {
		n := 0
		for _, v := range filtEachVec(ab) {
			n += int(v)
		}
		return n, true
	}
	return int(max(ab.ManaN, 1)), true
}

// paidGapTapExtra reports whether a TapsForMana static adds mana to this
// ability's tap: gorge's potentialTapExtra is zero for a non-tap ability and
// otherwise the largest extra over its productions. The table flag only says
// some card carries the static; the ability's own {T} and the source's
// reaching static decide (mana_tapextra.go).
func (s *State) paidGapTapExtra(k *prep, sl Slot, ab *kop.Ability) bool {
	if ab.Cost.Flags&kop.CostTap == 0 || k == nil || k.gen == nil || !k.gen.anyTapExtra {
		return false
	}
	pc := s.manaPCGen(k, sl)
	if pc == nil {
		return false
	}
	if _, hit := s.srcExtraTapped(k, sl, pc, true); hit {
		return true
	}
	_, hit := s.srcExtraTapped(k, sl, pc, false)
	return hit
}

// xBoundCountProof is the count arm's comparison, mirroring gorge's
// costOutrunsSupply: the covered census proves the announcement unpayable
// only when its total mana is short of the announced cost's total (CMC). It
// never reads a pip mask: a repeatable converter may change colours at a mana
// loss, so a colour-infeasible covered census is no proof while there is
// slack.
func xBoundCountProof(countEx *avail, cost *kop.ManaCost) bool {
	return countEx.total < cost.MV()
}

// xBoundCountCensus builds the count proof's census. An open production the
// bounded walk could not price (a granted any-colour unit, a lethal-pain
// fold, a genuinely unbounded source) leaves no proven upper bound, so the
// proof is unavailable; so does an incomplete per-tap record (potUnitsAvail).
func xBoundCountCensus(open bool, countEx *avail, units *[maxPotUnits]potUnit, life int) bool {
	return !open && potUnitsAvail(countEx, units, life)
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
	// cut: the subset holds one plain paid gap and no hybrid-fee one, so a
	// reducible cost is priced the way gorge's relaxed proof prices it
	// (paymentPlanRelaxProof's cutSelfFee): the gap's production comes off the
	// cost, and its fee is charged as generic against the sources without it,
	// sub[0]'s census (CR 601.2g-h: its own output cannot pay its own fee).
	// Two or more gaps may fund each other and keep the units above.
	cut gapProd
	has bool
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
	if d.gapNever != 0 {
		// a hybrid-fee filter has no relaxed alternative (its fee is floated
		// by the prefix search, which a relaxed subset does not model), so
		// gorge's census is not relaxable and its relaxed proof never runs
		// (paymentPlanCensusOf: !ok clears relaxable): no subset pricing
		// refuses the announcement and the over-bound stands, as for a
		// repeatable gap (Stolen by the Fae's X=1 over an Island, a Plains and
		// Flooded Grove; CR 605.3b, 601.2g)
		return
	}
	fees := d.gapFee
	for m := 0; m < 1<<n; m++ {
		dm := d
		dm.gapSkip, dm.gapN = ^uint8(m), 0
		dm.gapPrice = true
		var sub avail
		var pm [maxPotUnits]potUnit
		open := s.availPotentialFor(k, p, &sub, &dm, true, &pm)
		// Price the subset from the planner-shaped per-tap census, not the
		// over-bound pool: the pool's fold counts a source's second {T}
		// ability as a second mana (availPotentialFor's anyAddAb), so a
		// {1},{T} filter beside its own {T}: {C} read one mana too many and
		// the sweep offered an X-bound activation whose only announcement
		// the sources cannot pay (Rydia, Summoner of Mist). The included
		// gaps' productions are census units and their fees are charged on
		// top (gapSubsets.affords), so the record is the honest supply. An
		// open production or a repeatable admitted gap leaves no exact view:
		// keep the over-bound.
		if !open && !dm.gapRepeats {
			var exact avail
			if potUnitsAvail(&exact, &pm, int(s.pl[SeatIx(p)].life)) {
				sub = exact
			}
		}
		fee := 0
		for i := 0; i < n; i++ {
			if m>>i&1 != 0 {
				fee += int(fees[i])
			}
		}
		gs.sub[m] = gapSubset{av: sub, fee: fee}
		if bits.OnesCount(uint(m)) == 1 {
			if pr := d.gapProd[bits.TrailingZeros(uint(m))]; pr.plain {
				gs.sub[m].cut, gs.sub[m].has = pr, true
			}
		}
	}
	gs.n = 1 << n
}

// affords: some subset pays cost plus its fees.
func (gs *gapSubsets) affords(cx *kop.ManaCost) bool {
	for m := 0; m < gs.n; m++ {
		if sb := &gs.sub[m]; sb.has && cx.Hyb[0] == 0 && cx.Snow == 0 {
			if gs.sub[0].av.affordsCut(cx, sb) {
				return true
			}
			continue
		}
		c := *cx
		c.Generic = uint8(min(int(c.Generic)+gs.sub[m].fee, 255))
		if gs.sub[m].av.affords(&c, pipMaskOf(&c), 0) {
			return true
		}
	}
	return false
}

// affordsCut: some way the subset's gap makes its mana leaves a cost the
// sources a (the census without the gap) can pay with the gap's fee on top.
// Each unit pays a pip of its colour when the cost has one, else a generic
// (gorge pay.relaxedCutCost).
func (a *avail) affordsCut(cx *kop.ManaCost, sb *gapSubset) bool {
	return sb.cut.cutVecs(func(v [kop.NumManaKinds]uint8) bool {
		c := *cx
		spare := 0
		for i, n := range v {
			use := min(n, c.Pips[i])
			c.Pips[i] -= use
			spare += int(n - use)
		}
		c.Generic -= uint8(min(spare, int(c.Generic)))
		c.Generic = uint8(min(int(c.Generic)+sb.fee, 255))
		return a.affords(&c, pipMaskOf(&c), 0)
	})
}
