package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Cumulative upkeep with a coloured or hybrid mana cost (CR 702.24a; Phobian
// Phantasm, Yavimaya Ants, Arctic Nishoba). gorge's startCumulativeUpkeep
// scales the printed cost by the age counters (scaleCost: every coloured pip,
// the generic part and every hybrid pip repeated once per counter) and asks
// the pay-or-decline election once over the whole multiplied cost; a hybrid
// cost first announces each of its pips (cumulativeUpkeep.pips, the same
// announcement a trigger's own hybrid cost makes, cost_trighybrid.go).
//
// kgen marks such a cost with ExtCumulative B = kop.CumulativeScaled and
// leaves the UnlessPaid that follows at the cost of one counter. cumScale
// reads the age count at the pay instruction, and the sites that price the
// parked PayMana ask (execPay, resPayCost, canPayGen, trigCostPotentialPays,
// applyPayPip) multiply by it. The pay-or-decline ask keeps its canonical
// options (trigger_cost_pay, trigger_cost_decline). The life, sacrifice and
// discard costs of a cumulative upkeep need no marker: their UnlessPaid
// amount is the per-counter count times the AGE counters (kgen), and the pick
// is the ordinary echo-style object choice (echoActionPick).

// StallCumulativeCost: a scaled cumulative upkeep cost past what the packed
// ask holds -- a pip count above 7, a generic amount above 255, or more than
// four hybrid pips (kop.ManaCost.Hyb).
const StallCumulativeCost = 176

func init() {
	stallNames[StallCumulativeCost] = "cumulativecost"
}

// useCumScaled: some ability heads a scaled cumulative upkeep.
var useCumScaled = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtCumulative && in.B.N == kop.CumulativeScaled {
					return true
				}
			}
		}
	}
	return false
})

// cumScale is the age counters the pay instruction at the current pc scales
// its cost by: 0 unless the instruction before it is a scaled ExtCumulative.
func (s *State) cumScale(k *prep) int {
	pc := s.res.pc
	if k.gen == nil || !k.gen.uses.has(useCumScaled) || pc == 0 {
		return 0
	}
	if h := &s.prog(k)[pc-1]; h.Ext() != kop.ExtCumulative || h.B.N != kop.CumulativeScaled {
		return 0
	}
	return s.value(k, kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrAge)})
}

// cumPips scales the coloured pips of the pay instruction at pc by the age
// counters (unchanged outside a scaled cumulative upkeep).
func (s *State) cumPips(k *prep, pips int16) int16 {
	if n := s.cumScale(k); n > 0 {
		return scalePips(pips, n)
	}
	return pips
}

// scalePips multiplies each 3-bit pip count of p by n (kgen keeps every
// count times its maximum within 7).
func scalePips(p int16, n int) int16 {
	var out int16
	for i := 0; i < 5; i++ {
		out |= int16((int(p)>>(3*i)&7)*n) << (3 * i)
	}
	return out
}

// payManaHyb adds a PayMana F's hybrid pips to c, repeated once per age
// counter in a scaled cumulative upkeep (the unit's pips in printed order,
// then again).
func (s *State) payManaHyb(k *prep, f kop.FilterID, c *kop.ManaCost) {
	n := s.cumScale(k)
	if n <= 1 {
		kop.PayManaHyb(f, c)
		return
	}
	var unit kop.ManaCost
	kop.PayManaHyb(f, &unit)
	if nc := kop.PayManaCNum(f); nc != 0 {
		// the F's colourless pips (kop.PayManaCPip): once per age counter,
		// like the unit's coloured pips above
		c.Pips[kop.C] = uint8(int(c.Pips[kop.C]) + n*nc)
	}
	h := kop.PayManaHybN(f)
	for r := 0; r < n; r++ {
		for i := 0; i < h; i++ {
			j := r*h + i
			if j >= len(c.Hyb) {
				return
			}
			c.Hyb[j] = unit.Hyb[i]
			if unit.HybOrd>>i&1 != 0 {
				c.HybOrd |= 1 << j
			}
		}
	}
}

// cumulativeFits stalls a scaled cost the packed ask cannot hold. in is the
// ExtCumulative; the pay instruction is the one after it.
func (s *State) cumulativeFits(k *prep, in *kop.Instr) {
	if in.B.N != kop.CumulativeScaled || int(s.res.pc)+1 >= len(s.prog(k)) {
		return
	}
	pay := &s.prog(k)[s.res.pc+1]
	n := s.value(k, kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrAge)})
	if pay.A.K != kop.VConst || n*int(pay.A.N) > 255 || n*kop.PayManaHybN(pay.F) > len(kop.ManaCost{}.Hyb) ||
		n*kop.PayManaCNum(pay.F) > 255 {
		s.capFull(StallCumulativeCost)
		return
	}
	for i := 0; i < 5; i++ {
		if int(pay.B.N>>(3*i)&7)*n > 7 {
			s.capFull(StallCumulativeCost)
			return
		}
	}
}

// cumulativeAmount is the pay instruction's generic amount and packed pips as
// the cumulative upkeep ahead of it asks them: a scaled mana cost (the cost of
// one counter) times the age counters, and any amount held to the byte the ask
// carries (no pool pays more than 255).
func (s *State) cumulativeAmount(k *prep, kind, amt int, pips int16) (int, int16) {
	if !s.afterCumulative(k) {
		return amt, pips
	}
	if n := s.cumScale(k); kind == kop.PayMana && n > 0 {
		amt, pips = amt*n, scalePips(pips, n)
	}
	return min(amt, 255), pips
}
