package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The restricted fee pair: a source of a free restricted {T} ability of one
// mana of one colour beside a restricted mana-fee ability of one mana of any
// colour under the same restriction (Hydro-Channeler: "{T}: Add {U}" and
// "{1}, {T}: Add one mana of any color", instant and sorcery spells only).
//
// gorge's census lists the free {U} alone: a paid cost is no TierNormal
// alternative, and the census's fee walk skips a RestrictValid$ ability
// (mana_restrictedfeefund.go), so only the free alternative's colour counts
// in the remainder keys. Its cast window prices both alternatives of the one
// tap: the unit sorts with the fee units by its largest fee (pay
// planshape.go UnlessManaReachable's unitMaxFee) and either adds the {U} or
// pays the fee from the mana counted before it and adds one mana of any
// colour (CR 605.3b). The kernel keeps one record, the free one, whose view
// names the fee alternative's (pcard.restrPair): the planner and the census
// read the free {U}; restrSrcAvailD's fee pass prices the pair
// (restrFeePairAdd), and the fee gap's prefix activates the fee alternative
// (restrFeeStep).

// restrFeePairOf records v, the view of card id's restricted fee ability a,
// as the fee alternative of the card's free restricted record when the two
// make the pair, and reports whether it did (the fee ability then gets no
// record of its own: one tap, one unit).
func (g *genPrep) restrFeePairOf(c *kop.CardDef, id CardID, a *kop.Ability, v *pcard) bool {
	if restrFeeEmpty(v) || v.restrFeeCol != 0 || v.manaN != 1 || v.manaCol&colOrC != kop.ColAny || a.ManaEach ||
		a.AFlags&kop.AfManaChoice != 0 || a.ManaRestrictAb != 0 || a.AFlags&kop.AfManaNonSpell != 0 {
		return false
	}
	for i := range g.rs.recs {
		rec := &g.rs.recs[i]
		if rec.card != id || rec.kind != rkExt {
			continue
		}
		f := &g.rs.views[rec.view]
		fa := &c.Abilities[rec.ab]
		if !restrFeeEmpty(f) || f.restrPair != 0 || f.manaN != 1 || !oneColour(f.manaCol&kop.ColAny) || f.manaCol&kop.ColC != 0 ||
			f.restrLife != 0 || f.manaSac || f.manaChosen || fa.ManaRestrict != a.ManaRestrict || fa.ManaRestrictAb != 0 ||
			fa.AFlags&kop.AfManaNonSpell != 0 || fa.Trig.Cond != 0 || a.Trig.Cond != 0 {
			continue
		}
		g.rs.views = append(g.rs.views, *v)
		g.rs.views[rec.view].restrPair = uint16(len(g.rs.views))
		return true
	}
	return false
}

// restrFeePairAdd adds the pair whose free view is v to t, the payment's
// bound in restrSrcAvailD's fee pass: the free alternative's {U}, unless it
// leaves cost plus extra unpaid where the fee alternative -- its fee charged
// from plain, the unrestricted mana counted so far (restrFeeChargePlain), and
// funded as a fee unit's is (fund, wiped, poolLeft) -- pays it. A nil cost (a supply bound's read) takes the free
// alternative, the larger total.
func (s *State) restrFeePairAdd(k *prep, p uint8, d *payDesc, sl Slot, own, v *pcard, t, plain *avail, multi *[2]dynUnit, nm, lifeUsed *int,
	cost *kop.ManaCost, extra int, fund *int, wiped int, poolLeft *int) {
	free := *t
	s.restrSrcAddUnit(k, p, d, sl, own, v, rkExt, &free, multi, nm, lifeUsed, nil, 0)
	if cost == nil || free.affordsMulti(cost, extra, multi[:*nm]) {
		*t = free
		return
	}
	fv := &k.gen.rs.views[v.restrPair-1]
	fc := restrFeeCost(fv)
	pool, f := min(*poolLeft, fc.MV()), *fund
	if pool < fc.MV() {
		if wiped != 0 || f < fc.MV()-pool {
			*t = free // the census stands: no paid entry funds the fee
			return
		}
		f -= fc.MV() - pool
	}
	paid, pl := *t, *plain
	if !paid.restrFeeChargePlain(&pl, fc, cost, extra) {
		*t = free // no unrestricted mana pays the fee (CR 106.6): another pair's {U} cannot
		return
	}
	s.restrExtAdd(k, p, sl, fv, &paid, lifeUsed)
	if !paid.affordsMulti(cost, extra, multi[:*nm]) {
		*t = free
		return
	}
	*t, *plain, *fund, *poolLeft = paid, pl, f, *poolLeft-pool
}

// restrFeeGapColour is the colour fee gap sl's ability ab floats in the prefix
// (restrFeePrefix): a fixed production's own; for one mana of any colour (the
// pair's fee alternative) the colour ask's first answer, in the order gorge's
// script search tries them (the play's announced colours first, then W U B R
// G: scriptOrderAnswers), after which the plain walk lists play a, else the
// first answer.
func (s *State) restrFeeGapColour(k *prep, a Action, p uint8, sl Slot, ab *kop.Ability, fee int) int {
	if uint8(ab.ManaColors)&0x1f != uint8(kop.ColAny) || ab.ManaEach || ab.AFlags&kop.AfManaChoice != 0 {
		return bits.TrailingZeros8(uint8(ab.ManaColors) & 0x3f)
	}
	var order [5]uint8
	for c := range order {
		order[c] = uint8(c)
	}
	need, _ := s.playNeedPrinted(k, a)
	needFirst(order[:], need)
	var d Decision
	for _, c := range order {
		t := s.fork()
		t.restrFeeActivate(k, p, sl, ab, fee, int(c))
		t.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		listed := hasOption(&d, a)
		s.unfork()
		if listed {
			return int(c)
		}
	}
	return int(order[0])
}
