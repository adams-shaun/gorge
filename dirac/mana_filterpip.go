package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A coloured-fee filter of one fixed colour and a dynamic amount, gated by a
// counters condition: Evendo, Waking Haven's Station grant "{G}, {T}: Add {G}
// for each creature you control" (Uthros: {U}, artifacts), beside the land's
// free "{T}: Add {G}".
//
// gorge lists an AddAbility$ grant as a mana ability only while the LIVE pool
// pays its cost (mana_activation.go's AddAbility$ scan runs
// manaAbilityPayable whatever ignorePayable says), so with nothing floating
// the filter is in no census, no PotentialMana and no payment window; once
// {G} floats it is a member, its Amount$ X makes PotentialMana open in every
// colour (potentialAmount: indeterminate), and the census holds it as a paid
// gap whose fee the relaxed proof charges as generic one and the prefix
// search pays from the pool.
//
// So the kernel models it as a filter (filterTapMana) live only while its gate
// holds and the pool (with the restricted mana its activation may spend)
// holds its colour (filtGateOn): a gap of the census (scriptFilt) and an open
// source of the potential walk (potentialAdmits) only then. Its fee counts one
// in the fee arithmetic (filtFee), the gap step is legal only when that colour
// floats (filtPaid), and it is spent in that colour, restricted mana first
// (filtDo, filtPayPip).
//
// Not modelled: gorge's X-spell window (filtActivate, faceReachable) funds the
// fee from the mana made so far as a count, not by colour.

// pipFilt reports the coloured pip fee of a filter the kernel models: the
// gated dynamic one fixedPipFilt names, or any pip-fee mana ability
// kop.PipFeeManaAb admits (a first-only filter, Loot, the Pathfinder).
func pipFilt(a *kop.Ability) (col int, ok bool) {
	if col, ok := kop.PipFeeManaAb(a); ok {
		return col, true
	}
	return fixedPipFilt(a)
}

// fixedPipFilt reports whether a is that filter and its colour: exactly one
// coloured pip, no generic or hybrid part, the product that colour in a
// dynamic amount, and a gate (Trig.Cond) beside it.
func fixedPipFilt(a *kop.Ability) (col int, ok bool) {
	m := &a.Cost.Mana
	if a.Trig.Cond == 0 || a.AFlags != kop.AfManaDynamic || a.Cost.Flags != kop.CostTap || m.Generic != 0 || m.X != 0 || m.Hyb[0] != 0 {
		return 0, false
	}
	col = -1
	for c, n := range m.Pips {
		if n == 0 {
			continue
		}
		if n != 1 || col >= 0 || c > 4 {
			return 0, false
		}
		col = c
	}
	return col, col >= 0 && a.ManaColors == kop.ColorMask(1)<<uint(col)
}

// filtPip is pipFilt of the filter at sl.
func (s *State) filtPip(k *prep, sl Slot) (int, bool) {
	if !k.gen.anyFiltPip {
		return 0, false
	}
	return pipFilt(s.filtAb(k, sl))
}

// filtPipBit is the ability bit of sl's one-pip filter, 0 for none.
func (s *State) filtPipBit(k *prep, sl Slot) uint16 {
	c := s.cardAt(sl)
	if !k.gen.has(c, gfFilt) {
		return 0
	}
	if _, ok := s.filtPip(k, sl); !ok || k.gen.filt(c) >= 16 {
		return 0
	}
	return 1 << k.gen.filt(c)
}

// filtGateOn: the filter at sl is live. A counters-gated one (the Station
// grant) is no census gap while its condition fails, and a one-pip one is no
// mana ability of gorge's at all until the live pool pays its pip.
func (s *State) filtGateOn(k *prep, sl Slot) bool {
	a := s.filtAb(k, sl)
	if a.Cost.Flags&kop.CostOncePerGame != 0 && s.onceUsed(k, sl, a) {
		return false
	}
	pp := &s.perms[sl]
	if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: pp.obj, srcSl: sl, ctrl: pp.ctrl}) {
		return false
	}
	if col, ok := s.filtPip(k, sl); ok {
		return s.filtPipHeld(k, pp.ctrl, sl, col) >= 1
	}
	return true
}

// filtPipHeld is how much of colour col filter sl's fee may spend: p's pool
// and the restricted mana its activation may use.
func (s *State) filtPipHeld(k *prep, p uint8, sl Slot, col int) int {
	n := int(s.pl[SeatIx(p)].pool[col])
	if s.rbAny(p) {
		var held [kop.NumManaKinds]uint8
		s.rbUnits(p, s.filtFeeRestrM(k, p, sl), &held)
		n += int(held[col])
	}
	return n
}

// filtPaid: p's floating mana pays the fee of the filter at sl (a pip filter's
// {c} needs c itself, any other fee needs that much mana of any kind).
func (s *State) filtPaid(k *prep, p uint8, sl Slot) bool {
	if col, ok := s.filtPip(k, sl); ok {
		return s.filtPipHeld(k, p, sl, col) >= 1
	}
	return poolTotal(&s.pl[SeatIx(p)])+s.filtFeeRestrN(k, p, sl) >= s.filtFee(k, sl)
}

// filtPayPip pays the pip of the filter at sl: the restricted mana the
// activation may spend first (gorge carves those batches before ordinary
// mana), else the pool.
func (s *State) filtPayPip(k *prep, p uint8, sl Slot, col int) {
	if s.rbAny(p) && s.rbSpendCol(p, s.filtFeeRestrM(k, p, sl), col) {
		return
	}
	s.pl[SeatIx(p)].pool[col]--
}

// rbSpendCol spends one unit of colour col from p's batches in mask m and
// reports whether it found one.
func (s *State) rbSpendCol(p, m uint8, col int) bool {
	for i := range s.rb[SeatIx(p)] {
		if m&(1<<i) == 0 {
			continue
		}
		if b := &s.rb[SeatIx(p)][i]; b.n[col] > 0 {
			b.n[col]--
			s.rbCompact(p)
			if !s.rbAny(p) {
				s.g.poolArt[SeatIx(p)] &^= poolArtRestr
			}
			return true
		}
	}
	return false
}
