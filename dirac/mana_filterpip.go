package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A coloured-fee filter of one fixed colour and a dynamic amount, gated by a
// counters condition: Evendo, Waking Haven's Station grant "{G}, {T}: Add {G}
// for each creature you control" (Uthros: {U}, artifacts), beside the land's
// free "{T}: Add {G}". gorge's census sees the ability only while the grant is
// live (appendAvailableManaAbilitiesGate), as a paid gap whose fee its relaxed
// proof charges as generic one (the same arithmetic availFilt, faceReachable
// and filtActivate run), and its prefix search activates it only once the pool
// really pays {G}: the window lists an ability the floating mana pays
// (manaAbilityPayablePool, pay.ManaCostPayableFull, a real coloured check).
//
// So the kernel models it as a filter (filterTapMana) whose fee is one pip of
// the product's colour: it counts one in the fee arithmetic (filtFee) and the
// gap step is legal only when that colour floats (filtPaid), spending it from
// the restricted mana the activation may use first, then the pool (filtDo).
// Out of the gate it is no gap (scriptFilt).
//
// Not modelled: gorge's X-spell window (filtActivate, faceReachable) funds the
// fee from the mana made so far as a count, not by colour.

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

// filtPip is fixedPipFilt of the filter at sl.
func (s *State) filtPip(k *prep, sl Slot) (int, bool) {
	if !k.gen.anyFiltPip {
		return 0, false
	}
	return fixedPipFilt(s.filtAb(k, sl))
}

// filtGateOn: the filter at sl is live. A counters-gated one (the Station
// grant) is no census gap while its condition fails.
func (s *State) filtGateOn(k *prep, sl Slot) bool {
	a := s.filtAb(k, sl)
	return a.Trig.Cond == 0 || s.condEval(k, a.Trig.Cond, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl})
}

// filtPipHeld is how much of colour col filter sl's fee may spend: p's pool
// and the restricted mana its activation may use.
func (s *State) filtPipHeld(k *prep, p uint8, sl Slot, col int) int {
	n := int(s.pl[p&1].pool[col])
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
	return poolTotal(&s.pl[p&1])+s.filtFeeRestrN(k, p, sl) >= s.filtFee(k, sl)
}

// filtPayPip pays the pip of the filter at sl: the restricted mana the
// activation may spend first (gorge carves those batches before ordinary
// mana), else the pool.
func (s *State) filtPayPip(k *prep, p uint8, sl Slot, col int) {
	if s.rbAny(p) && s.rbSpendCol(p, s.filtFeeRestrM(k, p, sl), col) {
		return
	}
	s.pl[p&1].pool[col]--
}

// rbSpendCol spends one unit of colour col from p's batches in mask m and
// reports whether it found one.
func (s *State) rbSpendCol(p, m uint8, col int) bool {
	for i := range s.rb[p&1] {
		if m&(1<<i) == 0 {
			continue
		}
		if b := &s.rb[p&1][i]; b.n[col] > 0 {
			b.n[col]--
			s.rbCompact(p)
			if !s.rbAny(p) {
				s.g.poolArt[p&1] &^= poolArtRestr
			}
			return true
		}
	}
	return false
}
