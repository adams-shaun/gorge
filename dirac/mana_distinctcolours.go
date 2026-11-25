package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Firemind Vessel's "{T}: Add two mana of different colors" (Produced$
// Combo AnyDifferent). gorge resolves the production as a bounded choice of
// N DIFFERENT colours, one of each (engine/effects/mana_effect.go). The
// kernel keeps the same model here, outside the unrestricted Combo Any gap:
// the production is a distinct-colour allocation, so it coexists with the
// tap-another and filter gaps a Combo Any source would refuse (StallComboAnyMix).
//
// kgen leaves the ability's colour mask empty (Produced$ Combo AnyDifferent
// names no WUBRG letter), so the prep gives it no pcard slot (gen.go); the
// card's abilities are read directly. An untapped one is a census gap in
// gorge's planner: the prefix search and the payment windows activate it in
// battlefield order.

// tableAnyDifferent reports whether t holds a plain {T} AnyDifferent mana
// ability: the table needs a genPrep so scriptAny opens the scripted prefix
// search (the Combo Any gap's own registration, genneed.go).
func tableAnyDifferent(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if anyDifferentTapAb(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

var useAnyDifferent = newGenUse(tableAnyDifferent)

// anyDifferentTapAb reports whether a is an AnyDifferent mana ability: the
// plain {T} one (a mana-free cost of {T}, the choice flag the Combo$ parse
// sets and no colour, restriction, rider or program) or the same beside a
// counter removal from its source (Component Pouch, kop.AnyDifferentCtrManaAb).
func anyDifferentTapAb(a *kop.Ability) bool {
	if kop.AnyDifferentCtrManaAb(a) {
		return true
	}
	return a.Kind == kop.AbMana && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 &&
		a.AFlags == kop.AfManaChoice && a.ManaColors == 0 && a.ManaN > 0 && a.ManaRestrict == 0 && a.ManaRestrictAb == 0 &&
		!a.ManaEach && !a.ManaDeferred && a.ManaRider == kop.ManaRiderNone && len(a.Prog) == 0 &&
		a.Gate.Ctr == 0 && a.Trig.Cond == 0 && a.SplitGate == kop.SplitGateNone &&
		(a.ActZone == kop.ZoneNone || a.ActZone == kop.ZoneBattlefield)
}

// anyDifferentAbilityAt returns the plain {T} AnyDifferent mana ability of
// the permanent at sl, or nil.
func (s *State) anyDifferentAbilityAt(k *prep, sl Slot) *kop.Ability {
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		if anyDifferentTapAb(&cd.Abilities[j]) {
			return &cd.Abilities[j]
		}
	}
	return nil
}

// anyDifferentAt reports whether the permanent at sl is an untapped, usable
// source with an AnyDifferent mana ability whose counter cost, if it has one,
// the source can pay.
func (s *State) anyDifferentAt(k *prep, sl Slot) bool {
	if k.gen != nil && !k.gen.uses.has(useAnyDifferent) {
		return false // no card of the table has the ability
	}
	pp := &s.perms[sl]
	if pp.flags&pfTapped != 0 || pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) || s.activationSick(k, sl) {
		return false
	}
	a := s.anyDifferentAbilityAt(k, sl)
	return a != nil && (a.Cost.Flags&kop.CostRemoveCtr == 0 || pp.ctrN(s, int(a.Cost.Ctr)) >= int(a.Cost.N))
}

// anyDifferentPay pays AnyDifferent ability a's cost on the source at sl: its
// {T} and the counters its cost removes (CR 602.2b; the activation path's
// CostRemoveCtr, mana_window.go).
func (s *State) anyDifferentPay(k *prep, sl Slot, a *kop.Ability) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		s.perms[sl].seeCtr(s)
		s.subCtrFire(k, sl, int(a.Cost.Ctr), int(a.Cost.N)) // trig_counterremoved.go
	}
}

// anyDifferentGaps is p's usable AnyDifferent sources as a slot mask: the
// census gaps gorge's prefix search activates in battlefield order beside the
// gaps that fund a play (scriptSearch.visit).
//
//go:noinline
func (s *State) anyDifferentGaps(k *prep, p uint8) slotMask {
	var m slotMask
	if k.gen == nil || !k.gen.uses.has(useAnyDifferent) {
		return m // no card of the table has the ability (useAnyDifferent; anyDifferentAbilityAt reads the card's own)
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		if s.anyDifferentAt(k, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// anyDifferentOpen reports whether p controls an untapped, usable
// AnyDifferent source. Gorge's potential bound now prices it as one of each
// colour (rules potential.go potentialAnyDifferentEach), not open.
//
//go:noinline
func (s *State) anyDifferentOpen(k *prep, p uint8) bool {
	if k.gen == nil || !k.gen.uses.has(useAnyDifferent) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		if s.anyDifferentAt(k, m.first()) {
			return true
		}
	}
	return false
}

// windowGapListed is windowPick's gap test: a Tender, restricted or Combo Any
// source the window lists (windowGap), or an AnyDifferent source, which the
// planner lists in its battlefield place in every window (a trigger cost's, a
// Mystic Remora's pay-{4} ask), not only the potential one.
func (s *State) windowGapListed(k *prep, p uint8, o Slot) bool {
	return k.gen != nil && (k.gen.anyWindowGap && s.windowGap(k, p, o) || s.anyDifferentAt(k, o))
}

// anyDifferentUnits is how many mana an AnyDifferent activation adds: its
// amount, at most one of each of the five colours.
func anyDifferentUnits(a *kop.Ability) uint8 {
	return uint8(min(max(int(a.ManaN), 1), 5))
}

// anyDifferentAnswers is the number of distinct-colour allocations of n
// units: scriptAnswers' odometer answers, with a colour repeated dropped
// exactly as Decision.Validate drops it.
func anyDifferentAnswers(n int) int {
	total := caAnswerCount(n)
	count := 0
	for i := 0; i < total; i++ {
		if !anyDifferentRepeats(i, n) {
			count++
		}
	}
	return count
}

// anyDifferentRepeats reports whether odometer answer i of n units names a
// colour twice.
func anyDifferentRepeats(i, n int) bool {
	var seen [5]bool
	for u := 0; u < n; u++ {
		d := caDigit(i, n, u)
		if d < 0 || d >= 5 || seen[d] {
			return true
		}
		seen[d] = true
	}
	return false
}

// anyDifferentRank is the odometer index of the r'th distinct-colour answer
// gorge's search tries (scriptOrderAnswers): answers naming a colour the play
// announces first, then the rest, each half in odometer order.
func anyDifferentRank(n int, need *[kop.NumManaKinds]bool, r int) int {
	total := caAnswerCount(n)
	for _, first := range [2]bool{true, false} {
		for i := 0; i < total; i++ {
			if anyDifferentRepeats(i, n) || caNames(i, n, need) != first {
				continue
			}
			if r == 0 {
				return i
			}
			r--
		}
	}
	return 0
}

// anyDifferentTap activates the AnyDifferent source at sl with odometer
// answer ans: the tap, then one mana of each distinct colour the answer names.
func (s *State) anyDifferentTap(k *prep, p uint8, sl Slot, ans int) {
	a := s.anyDifferentAbilityAt(k, sl)
	if a == nil {
		return
	}
	n := max(int(a.ManaN), 1)
	if n > 5 {
		n = 5
	}
	s.anyDifferentPay(k, sl, a)
	var got [5]int
	for u := 0; u < n; u++ {
		got[caDigit(ans, n, u)]++
	}
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}

// windowAnyDifferentTap activates AnyDifferent source sl in a payment window:
// the tap, then N DISTINCT colours the cost still owes (else the lowest
// listed), as windowComboTap does for a Combo Any source.
func (s *State) windowAnyDifferentTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) {
	a := s.anyDifferentAbilityAt(k, sl)
	if a == nil {
		return
	}
	n := max(int(a.ManaN), 1)
	if n > 5 {
		n = 5
	}
	s.anyDifferentPay(k, sl, a)
	mask := uint8(kop.ColAny)
	if clamp {
		if want, decided := s.windowUseful(k, p, useful); decided && want&mask != 0 {
			mask &= want
		}
	}
	var owed [kop.NumManaKinds]uint8
	if cost != nil && !noOwed {
		owed = cost.Pips
		for c := range owed {
			owed[c] -= min(owed[c], s.pl[p].pool[c])
		}
	}
	var used [5]bool
	var got [5]int
	for i := 0; i < n; i++ {
		col := -1
		for c := 0; c < 5; c++ {
			if mask&(1<<c) != 0 && !used[c] && owed[c] > 0 {
				col = c
				owed[c]--
				break
			}
		}
		if col < 0 {
			for c := 0; c < 5; c++ {
				if mask&(1<<c) != 0 && !used[c] {
					col = c
					break
				}
			}
		}
		if col < 0 {
			col = bits.TrailingZeros8(mask)
		}
		used[col] = true
		got[col]++
	}
	for col, c := range got {
		if c > 0 {
			s.poolAdd(p, col, c)
			s.artTag(k, p, sl, col)
		}
	}
	s.tapExtraStall(k, sl)
}
