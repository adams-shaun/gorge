package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Crystal Vein: {T}: Add {C}, then {T}, Sacrifice this permanent: Add
// {C}{C}. gorge's census unit for the source holds both alternatives
// (payment_plan.go paymentPlanManaUnitsOnlyCompute, pay planalts.go) and the
// planner takes the sacrificing one only when the tap alone cannot pay
// (gorge's last-resort tier; the kernel's irreversible cost, unitIrr, ranks
// it and runOrder runs it last). The kernel lowers it as a pair view beside
// the gated pair (mana_gate.go): pc.wideN/pc.wideCol carry the larger
// production to the affordability bound (srcYield) and the potential walk
// (gatePairPotential, legal.go), while the census stays the plain {T}
// ability, and sacPairUnit registers the two unit types with the wider one
// marked utype.sac, so its payment taps, fires the sacrifice and leaves with
// the mana (mana.go's ty.sac arms), mirroring restrPairUnit's registration
// (mana_restrictedsource.go) rather than the restricted semantics.

// prepManaSacPair records the kop.SacSelfPairManaAb wider alternative beside
// the card's plain {T} mana ability (kop.SecondSacSelfPairManaAb, the shape
// dirac/kgen lower_multimana.go accepts): pc.wideN/pc.wideCol carry the
// larger production and gfManaSacPair marks the card for sacPairUnit. The
// census ability must be the plain one the pair ranks over; every other
// shape (a chosen, dynamic, restricted or sacrificing census) keeps its own
// model and takes no pair.
func prepManaSacPair(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	first, wider := -1, -1
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if first < 0 {
			first = j
			continue
		}
		if kop.SacSelfPairManaAb(a) {
			if wider >= 0 {
				return // three mana abilities: not the lowered pair shape
			}
			wider = j
		} else {
			return // another mana ability beside the pair
		}
	}
	if wider < 0 || !kop.SecondSacSelfPairManaAb(c, first, wider) || int(pc.manaAb) != first || pc.manaSac || pc.manaDyn || pc.manaSacOnly {
		return
	}
	a := &c.Abilities[wider]
	pc.wideN, pc.wideCol = a.ManaN, a.ManaColors&colOrC
	g.setFlag(id, gfManaSacPair)
}

// sacPairUnit registers Crystal Vein's source as the pair of unit types
// restrPairUnit's mechanism carries: the census tap type ta and the wider
// sacrificing type tb, cross-linked so the search picks one per payment
// (bestTy, ty.pair), the sacrificing one carrying utype.sac so planPay
// taps, sacrifices and leaves with the mana (its ty.sac arms) and
// unitIrr's irreversible cost keeps it out of a plan the tap can pay. The
// source's own sacrifice reserved by the cost (pk.kept) is excluded by the
// caller: then only the tap alternative stands, the ordinary unit.
func (s *State) sacPairUnit(k *prep, sl Slot, v *pcard, pk *planner, i int) bool {
	own := s.manaPCGen(k, sl)
	ma, na := s.srcMaskAt(own, sl), own.manaN
	mb, nb := uint8(v.wideCol), v.wideN
	if ma&^mb != 0 || own.manaAb < 0 {
		return false // not the lowered pair shape: the larger alternative holds the census one
	}
	cr := s.isCreature(k, sl)
	dmk := srcMask(v) // the colours both alternatives rank over
	var xv [kop.NumManaKinds]uint8
	var xm, xt uint8
	if k.gen != nil && k.gen.anyTapExtra {
		xv = s.srcExtra(k, sl, own) // the same fixed extra on either alternative
		for c, n := range xv {
			if n > 0 {
				xm |= 1 << c
				xt += n
			}
		}
	}
	ta, tb := -1, -1
	for t := 0; t < pk.nt && ta < 0; t++ {
		a := &pk.types[t]
		if a.pair == 0 || a.alt || a.rb || a.sac || a.pain != 0 || a.mask != ma || a.n != na || a.dmask != dmk || a.creat != cr || a.xv != xv {
			continue
		}
		if b := &pk.types[a.pair-1]; b.sac && !b.rb && b.pain == 0 && b.mask == mb && b.n == nb && b.fee == 0 && b.feeCol == 0 {
			ta, tb = t, int(a.pair)-1
		}
	}
	if ta < 0 {
		if pk.nt+2 > maxTypes || pk.nu >= maxUnits {
			s.capFull(StallRestrSrc)
			return true
		}
		fl := uint8(bits.OnesCount8(dmk | xm))
		ta, tb = pk.nt, pk.nt+1
		pk.nt += 2
		pk.types[ta] = utype{mask: ma, dmask: dmk, n: na, creat: cr, flex: fl, pair: uint8(tb) + 1, xv: xv, xmask: xm, xt: xt}
		pk.types[tb] = utype{mask: mb, dmask: dmk, n: nb, creat: cr, flex: fl, pair: uint8(ta) + 1, alt: true, sac: true, irr: unitIrr(true, cr, false), xv: xv, xmask: xm, xt: xt}
		pk.maxXt = pk.maxXt || xt != 0
		pk.maxN = max(pk.maxN, int(max(na, nb))+int(xt))
		pk.pairs = true
	}
	pk.types[ta].avail++
	pk.types[tb].avail++
	pk.units[pk.nu] = sl
	pk.rankUnit(s, i, sl)
	pk.utyp[pk.nu] = uint8(ta)
	pk.rkind[pk.nu] = 0
	pk.rAb[pk.nu] = v.manaAb
	pk.nu++
	for c := 0; c < 5; c++ {
		if (dmk|xm)&(1<<c) != 0 {
			pk.colSrc[c]++
		}
	}
	return true
}
