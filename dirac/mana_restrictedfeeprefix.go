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
	if hasOption(&d, a) {
		return
	}
	pd := s.grantPayDesc(k, a)
	fund, wiped := s.restrFeeFund(k, p)
	if wiped != 0 {
		return
	}
	for i := 0; i < maxFeePrefix; i++ {
		sl, ab, fee := s.restrFeeGap(k, p, &pd)
		if sl == NoSlot {
			return
		}
		pl := &s.pl[SeatIx(p)]
		for poolTotal(pl)+s.filtFeeRestrN(k, p, sl) < fee {
			if fund == 0 || !s.restrFeeFundTap(k, p, sl) {
				return
			}
			fund--
		}
		s.restrFeeActivate(k, p, sl, ab, fee)
		s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		if hasOption(&d, a) {
			return
		}
	}
}

// restrFeeGap is p's first untapped fee gap play pd may spend, in battlefield
// order: its slot, ability and generic fee. NoSlot when none; a fee with a
// coloured pip is not scripted.
func (s *State) restrFeeGap(k *prep, p uint8, pd *payDesc) (Slot, *kop.Ability, int) {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.ctrlM[p], order[:], 0)
	for _, sl := range order[:n] {
		c := s.cardAt(sl)
		if k.gen.flags[c]&gfRestrSrc == 0 || !s.restrUsable(k, sl) {
			continue
		}
		v, kind, ab := s.restrView(k, p, sl, s.manaPCGen(k, sl), pd)
		if v == nil || kind != rkExt || restrFeeEmpty(v) || v.restrFeeCol != 0 {
			continue
		}
		return sl, ab, int(v.restrFee)
	}
	return NoSlot, nil, 0
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
		col := bits.TrailingZeros8(m)
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		s.poolAdd(p, col, int(max(pc.manaN, 1)))
		s.addExtra(k, p, sl, pc)
		s.artTag(k, p, sl, col)
		return true
	}
	return false
}

// restrFeeActivate activates fee gap sl: the fee paid from the pool, then its
// restricted production floating (the window's order, CR 605.3b).
func (s *State) restrFeeActivate(k *prep, p uint8, sl Slot, ab *kop.Ability, fee int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	dc, dn := s.deferSnap(p) // a deferred source's unit the fee spends is gone (mana_deferred.go)
	s.filtFeeSpend(k, p, sl, fee)
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	col := bits.TrailingZeros8(uint8(ab.ManaColors) & 0x3f)
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
