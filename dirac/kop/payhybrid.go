package kop

// PayMana's F (an OpUnlessPaid / OpIfPaid of pay kind PayMana) is no filter:
// it packs what a trigger's own cost pays beside the generic (A) and the
// coloured pips (B). Bits 0-5 are the life paid with the mana (Zoraline's
// "{W}{B} and 2 life"); bits 6-15 hold up to PayHybMax hybrid pips (Emiel
// the Blessed's {G/W}), five bits each: 0 = none, else 1 + 5*first + second,
// the pip's two WUBRG colour indexes in printed order (gorge offers and
// resolves a pip's first printed colour first, ManaCost.HybOrd), or the
// PayManaCPip marker for a colourless {C} pip (Ulalek's trigger cost
// "pay {C}{C}").
const (
	PayHybMax    = 2
	payFLifeBits = 6
	payFLifeMask = 1<<payFLifeBits - 1
	// PayManaCPip is the pip-field marker of a colourless pip paid with
	// colourless mana (CR 107.4a): 1+5a+b never reaches it (a, b < 5), and it
	// is not an announcement pip -- a {C} pip has no face to announce (gorge
	// pay.AnnouncePip lists only hybrid, twobrid and Phyrexian pips), so it
	// folds straight into the cost's Pips[C] (the pool pays it from its
	// colourless units, as a printed {C} pip's cast payment does).
	PayManaCPip = 26
)

// PayManaF packs life and the hybrid pips (each its colour indexes in
// printed order, or {C, C} for a colourless pip); ok is false past the
// encoding.
func PayManaF(life int, hyb [][2]uint8) (FilterID, bool) {
	if life < 0 || life > payFLifeMask || len(hyb) > PayHybMax {
		return 0, false
	}
	f := FilterID(life)
	for i, h := range hyb {
		if h[0] == 5 && h[1] == 5 {
			f |= FilterID(PayManaCPip) << (payFLifeBits + 5*i)
			continue
		}
		if h[0] > 4 || h[1] > 4 || h[0] == h[1] {
			return 0, false
		}
		f |= FilterID(1+5*h[0]+h[1]) << (payFLifeBits + 5*i)
	}
	return f, true
}

// PayManaLife is the life a PayMana F pays beside the mana.
func PayManaLife(f FilterID) int { return int(f & payFLifeMask) }

// PayManaHyb adds a PayMana F's hybrid pips to c (Hyb and HybOrd) and its
// colourless pips to Pips[C] (a {C} pip announced no face, PayManaCPip).
func PayManaHyb(f FilterID, c *ManaCost) {
	for i := 0; i < PayHybMax; i++ {
		v := int(f>>(payFLifeBits+5*i)) & 31
		if v == 0 {
			return
		}
		if v == PayManaCPip {
			c.Pips[C]++
			continue
		}
		a, b := (v-1)/5, (v-1)%5
		c.Hyb[i] = ColorMask(1<<a | 1<<b)
		if a > b {
			c.HybOrd |= 1 << i
		}
	}
}

// PayManaHybN is how many announcement hybrid pips a PayMana F carries: the
// pips with a face to announce, never its colourless {C} pips (a cost of
// nothing but {C} pips poses no pip ask).
func PayManaHybN(f FilterID) int {
	n := 0
	for i := 0; i < PayHybMax; i++ {
		v := int(f >> (payFLifeBits + 5*i) & 31)
		if v == 0 || v == PayManaCPip {
			return n
		}
		n++
	}
	return n
}

// PayManaCNum is how many colourless {C} pips a PayMana F carries.
func PayManaCNum(f FilterID) int {
	n := 0
	for i := 0; i < PayHybMax; i++ {
		if int(f>>(payFLifeBits+5*i))&31 == PayManaCPip {
			n++
		}
	}
	return n
}

// IsPayMana reports whether in is a pay instruction of kind PayMana, whose
// F is the packed life and hybrid pips above, never a FilterID.
func (in *Instr) IsPayMana() bool {
	return (in.Op == OpUnlessPaid || in.Op == OpIfPaid || in.Op == OpUnlessSwitched) && int(in.C&0xF) == PayMana
}

// InstrFilter is in's filter for a walk over every instruction's filter: F,
// or 0 on a PayMana or PayManaX pay instruction (PayMana's F packs life and
// hybrid pips; PayManaX's packs fixed coloured pips beside the announced {X},
// dirac/kgen/lower_triggerxcost.go and dirac/trig_maypayx.go trigXCostAt).
func InstrFilter(in *Instr) FilterID {
	if in.IsPayMana() || (in.Op == OpUnlessPaid || in.Op == OpIfPaid || in.Op == OpUnlessSwitched) && int(in.C&0xF) == PayManaX {
		return 0
	}
	return in.F
}
