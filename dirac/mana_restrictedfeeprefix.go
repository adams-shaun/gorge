package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The restricted fee gap's prefix: a restricted mana ability whose cost is
// {N}, {T} and which has no census alternative (Cormela, Glamour Thief's {1},
// {T}: Add {U}{B}{R}; rkExt) is a gap in gorge's payment census
// (mana_restrictedfeefund.go). A play the census's lands alone do not pay is
// proven by the prefix search (engine/rules potential_script.go
// potentialPrefixScript), which activates it at priority, before the play: the
// fee is not payable from an empty pool, so the search first taps the first
// covered source in battlefield order (the funding, at most census.fees() of
// them), then activates the gap, paying its fee from the floating mana, and
// the restricted production floats. A targeted spell's target ask comes after
// that, so the mana is on the table at the ask (an untargeted play shows no
// difference: the post-push planner taps the same mana). The kernel prices the
// unit in restrSrcAvailD's fee pass and would tap it in the post-push planner;
// this prefix plays the search's script instead.

// availNoFeeUnit is an availX mode bit for restrFeePrefix's "does the census
// alone list this play" probe: restrSrcAvailD's fee pass counts no fee unit.
const availNoFeeUnit uint16 = 32768

// maxFeePrefix bounds the fee units one prefix activates.
const maxFeePrefix = 4

// restrFeeGapAny reports whether a restricted fee gap's prefix may run: the
// full surface's cast window prices fee units (the autopay surface's census
// lists none) and no other gap kind is scripted on the table.
func (s *State) restrFeeGapAny(k *prep) bool {
	return k.gen != nil && k.gen.rs != nil && k.gen.rs.fee && s.cfg&cfgGorgeAutopay == 0 && !s.scriptAny(k)
}

// restrFeePrefix plays gorge's script for play a over p's restricted fee
// gaps: nothing when the census's lands list the play alone, else each
// gap in battlefield order funded from the pool or the first covered source and
// activated, until the plain walk lists the play. A source it cannot tap
// as gorge's search does (a choice of colour, a restricted unit) ends the
// prefix: the play stays on the offer for the post-push planner.
func (s *State) restrFeePrefix(k *prep, a Action, p uint8) {
	var d Decision
	s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
	pd := s.grantPayDesc(k, a)
	if hasOption(&d, a) {
		return
	}
	if s.restrComboPrefix(k, p, a, &pd) {
		return
	}
	fund, wiped := s.restrFeeFund(k, p)
	if wiped != 0 {
		return
	}
	for i := 0; i < maxFeePrefix; i++ {
		sl, ab, fee, ok := s.restrFeeStep(k, p, a, &pd)
		if !ok {
			return
		}
		pl := &s.pl[SeatIx(p)]
		for poolTotal(pl)+s.filtFeeRestrN(k, p, sl) < fee {
			if fund == 0 || !s.restrFeeFundTap(k, p, sl) {
				return
			}
			fund--
		}
		s.restrFeeActivate(k, p, sl, ab, fee, s.restrFeeGapColour(k, a, p, sl, ab, fee))
		s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		if hasOption(&d, a) {
			return
		}
	}
}

// restrFeeGapView is the eligible gap of the source at sl for play pd: its
// record's view and ability (a pair's free view, its restrPair naming the fee
// alternative), or false when the source is no gap pd may spend. A coloured
// fee is not scripted. A Great Hall's combo pair (rkWiden, restrComboWide) is
// a gap too: its census {C} covers none of the fee ability (gorge's census
// reads TierNormal alternatives only).
func (s *State) restrFeeGapView(k *prep, p uint8, sl Slot, pd *payDesc) (v *pcard, ab *kop.Ability, ok bool) {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfRestrSrc == 0 || !s.restrUsable(k, sl) {
		return nil, nil, false
	}
	v, kind, ab := s.restrView(k, p, sl, s.manaPCGen(k, sl), pd)
	if v == nil || (kind != rkExt && !(kind == rkWiden && restrComboWide(v))) || v.restrFeeCol != 0 {
		return nil, nil, false
	}
	if v.restrPair == 0 && restrFeeEmpty(v) {
		return nil, nil, false // a fee-only gap needs a fee; a pair's free view has none
	}
	return v, ab, true
}

// restrFeeStep is restrFeePrefix's next gap activation: the first untapped gap
// source play pd may spend, in battlefield order. A pair's free alternative
// comes first -- gorge's plain search taps a gap's zero-cost ability before any
// fee -- and only a pair with no later gap to continue the script pays its fee,
// when the free step alone does not list the play (restrPairFreeLists). A
// fee-only gap pays its fee.
func (s *State) restrFeeStep(k *prep, p uint8, a Action, pd *payDesc) (sl Slot, ab *kop.Ability, fee int, ok bool) {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], order[:], 0)
	for i, sl := range order[:n] {
		v, ab, ok := s.restrFeeGapView(k, p, sl, pd)
		if !ok {
			continue
		}
		if v.restrPair == 0 {
			return sl, ab, int(v.restrFee), true
		}
		if s.restrGapAfter(k, p, order[i+1:n], pd) || s.restrPairFreeLists(k, p, a, sl, ab) {
			return sl, ab, 0, true
		}
		// the free step cannot list the play and no later gap can: the script
		// backtracks to this source's fee (mana_restrictedfeepair.go)
		fv := &k.gen.rs.views[v.restrPair-1]
		return sl, &k.t.Cards[s.cardAt(sl)].Abilities[fv.manaAb], int(fv.restrFee), true
	}
	return NoSlot, nil, 0, false
}

// restrGapAfter reports whether a later gap source can continue the script of
// a pair whose free alternative was just taken.
func (s *State) restrGapAfter(k *prep, p uint8, rest []Slot, pd *payDesc) bool {
	for _, sl := range rest {
		if _, _, ok := s.restrFeeGapView(k, p, sl, pd); ok {
			return true
		}
	}
	return false
}

// restrPairFreeLists reports whether sl's free alternative alone lists play a:
// the last pair's script step is its fee only when the free step leaves the
// play unlisted. It runs on a fork, like restrFeeGapColour's colour probes.
func (s *State) restrPairFreeLists(k *prep, p uint8, a Action, sl Slot, ab *kop.Ability) bool {
	t := s.fork()
	t.restrFeeActivate(k, p, sl, ab, 0, bits.TrailingZeros8(uint8(ab.ManaColors)&0x3f))
	var d Decision
	t.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
	listed := hasOption(&d, a)
	s.unfork()
	return listed
}

// restrFeeFundTap taps the first covered source of p, in battlefield order,
// into the pool: the funding gorge's search taps for a gap's fee. It reports
// false when that source is not a fixed-colour plain tap the kernel replays
// (the search would branch on a colour ask, or the source is restricted).
func (s *State) restrFeeFundTap(k *prep, p uint8, gap Slot) bool {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], order[:], 0)
	for _, sl := range order[:n] {
		c := s.cardAt(sl)
		if sl == gap || k.gen.flags[c]&gfRestrSrc != 0 {
			continue
		}
		pc := s.manaPCGen(k, sl)
		if !s.canTapForManaPC(k, sl, pc) {
			continue
		}
		m := s.srcMaskAt(pc, sl) & 0x3f
		if m == 0 || m&(m-1) != 0 || pc.manaMulti || pc.manaDyn || pc.painCol != 0 || pc.manaRefl {
			return false
		}
		s.restrFeeTapCensus(k, p, sl, pc, bits.TrailingZeros8(m))
		return true
	}
	return false
}

// restrFeeTapCensus taps sl's census ability (view pc, colour col) into p's
// pool, as gorge's search activates it.
func (s *State) restrFeeTapCensus(k *prep, p uint8, sl Slot, pc *pcard, col int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	s.poolAdd(p, col, int(max(pc.manaN, 1)))
	s.addExtra(k, p, sl, pc)
	s.artTag(k, p, sl, col)
}

// restrComboPrefix plays gorge's search over two or more Great Halls (combo
// fee pairs, restrComboWide), p's only fee gaps: the search tries a gap's {T}: Add
// {C} before its fee ability, depth first in battlefield order, so its first
// success taps every Hall but the last for {C} and activates the last one's
// fee from that pool, with the first combination answer (the play's colours
// first: wideEngineAnswer) the plain walk then lists the play on (engine/rules
// potential_script.go scriptSearch.visit; no funding is spent: every tap is
// a gap's). It reports whether it handled the play; a board it does not model
// (another fee gap kind, one Hall) takes restrFeePrefix's funded walk. When
// no answer lists the play the first stays floating (gorge's deeper branches,
// two fees, are not modelled).
func (s *State) restrComboPrefix(k *prep, p uint8, a Action, pd *payDesc) bool {
	var gaps [MaxPerm]Slot
	var order [MaxPerm]Slot
	n, ng := s.battlefieldOrder(p, s.ctrlM[p], order[:], 0), 0
	fee := 0
	for _, sl := range order[:n] {
		if k.gen.flags[s.cardAt(sl)]&gfRestrSrc == 0 || !s.restrUsable(k, sl) {
			continue
		}
		own := s.manaPCGen(k, sl)
		v, kind, _ := s.restrView(k, p, sl, own, pd)
		if v == nil || restrFeeEmpty(v) {
			continue
		}
		if kind != rkWiden || !restrComboWide(v) || v.restrFeeCol != 0 || !s.canTapForManaPC(k, sl, own) {
			return false // another fee gap kind: the funded walk's
		}
		gaps[ng], fee = sl, int(v.restrFee)
		ng++
	}
	pl := &s.pl[SeatIx(p)]
	if ng < 2 || poolTotal(pl)+ng-1 < fee {
		return false
	}
	for _, sl := range gaps[:ng-1] {
		s.restrFeeTapCensus(k, p, sl, s.manaPCGen(k, sl), bits.TrailingZeros8(uint8(kop.ColC)))
	}
	last := gaps[ng-1]
	_, _, ab := s.restrView(k, p, last, s.manaPCGen(k, last), pd)
	s.perms[last].flags |= pfTapped
	s.touchSl(last)
	s.manaTapped(k, last)
	dc, dn := s.deferSnap(p)
	s.filtFeeSpend(k, p, last, fee)
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	need := s.playNeed(k, a)
	rb, pool := s.rb[SeatIx(p)], pl.pool
	var d Decision
	for c := 0; c < wideAnswers; c++ {
		x, y := wideAnswer(wideEngineAnswer(need, c))
		s.rb[SeatIx(p)], pl.pool = rb, pool
		if !s.floatRestrAb(k, p, last, ab, x, 1) || !s.floatRestrAb(k, p, last, ab, y, 1) {
			s.capFull(StallRestrSrc) // never a silent spill
			return true
		}
		s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		if hasOption(&d, a) {
			return true
		}
	}
	s.rb[SeatIx(p)], pl.pool = rb, pool
	x, y := wideAnswer(wideEngineAnswer(need, 0))
	if !s.floatRestrAb(k, p, last, ab, x, 1) || !s.floatRestrAb(k, p, last, ab, y, 1) {
		s.capFull(StallRestrSrc)
	}
	return true
}

// restrFeeActivate activates fee gap sl: the fee paid from the pool, then its
// restricted production of colour col floating (the window's order, CR
// 605.3b).
func (s *State) restrFeeActivate(k *prep, p uint8, sl Slot, ab *kop.Ability, fee, col int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	dc, dn := s.deferSnap(p) // a deferred source's unit the fee spends is gone (mana_deferred.go)
	s.filtFeeSpend(k, p, sl, fee)
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	if !s.floatRestrAb(k, p, sl, ab, col, s.restrAbN(k, sl, ab)) {
		s.capFull(StallRestrSrc) // never a silent spill
	}
}

// restrNoColour is the plan's last-resort-or-gap test for a unit: lastResort
// (a sacrificed source, a source that stays tapped, a life payment), or a
// restricted fee unit with no census alternative (rkExt). Gorge's census never
// lists a restricted fee ability (mana_restrictedfeefund.go), so keys 6 and 7
// count no colour of the unit (engine/rules/pay planalts.go Alt.Colours reads
// TierNormal alternatives only).
func restrNoColour(lastResort bool, fee, feeCol, rkind uint8) bool {
	return lastResort || (fee != 0 || feeCol != 0) && rkind == rkExt
}
