package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A wide any-colour restricted fee gap (Orb of Dragonkind's "{1}, {T}: Add two
// mana in any combination of colors", Dragon spells and abilities only) in the
// restricted fee prefix (restrFeePrefix). Gorge's script search answers the
// source's colour ask with the first pair of colours after which the plain walk
// lists the play, the pairs in the order scriptOrderAnswers gives (the play's
// announced colours first), exactly as restrComboPrefix does for a Great Hall's
// combo pair: a prefix that floated the first colour twice paid a {1}{R} pump
// with no red (CR 106.6, 601.2b, 605.3b).

// restrFeeWideAb reports whether the fee gap ability ab makes two mana of any
// combination of colours.
func restrFeeWideAb(ab *kop.Ability) bool {
	return ab.AFlags&kop.AfManaChoice != 0 && ab.ManaN == 2 && ab.ManaColors&kop.ColAny == kop.ColAny
}

// windowFeeWide reports whether sl is p's untapped restricted fee gap whose
// ability makes two mana of any combination of colours (Orb of Dragonkind's
// "{1}, {T}: Add two mana in any combination of colors", Dragon spells and
// abilities only): an activate option of gorge's payment window when the fee
// is payable from the live pool (the windows keep manaAbilityPayable's gate,
// engine/rules mana_activation.go) and the payment may spend the production
// (CR 106.6). The walk takes it there (windowFeeWideTap).
func (s *State) windowFeeWide(k *prep, p uint8, sl Slot) bool {
	if k.gen == nil || !k.gen.anyRestrSrc || !s.restrUsable(k, sl) {
		return false
	}
	d := s.payDescNow(NoObj)
	v, kind, ab := s.restrView(k, p, sl, s.manaPCGen(k, sl), &d)
	if v == nil || kind != rkExt || v.restrFeeCol != 0 || restrFeeEmpty(v) || !restrFeeWideAb(ab) {
		return false
	}
	fee := restrFeeCost(v)
	return s.windowPoolPays(p, &fee, false)
}

// windowFeeWideTap activates the restricted fee gap sl in the payment's
// window as gorge's window answer does: the tap, the fee paid from the pool
// (restrFeeActivate's order, CR 605.3b), then each of the two mana floating
// restricted (State.rb) for a colour the cost still owes, else the lowest
// listed (payexec.WindowChoices, windowWideTap's answer model). Mana the
// byte cannot hold as restricted floats plain (held).
func (s *State) windowFeeWideTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool, held *[kop.NumManaKinds]uint8) {
	d := s.payDescNow(NoObj)
	v, _, ab := s.restrView(k, p, sl, s.manaPCGen(k, sl), &d)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	dc, dn := s.deferSnap(p) // a deferred source's unit the fee spends is gone (mana_deferred.go)
	s.filtFeeSpend(k, p, sl, int(v.restrFee))
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	mask := uint8(ab.ManaColors)
	// askManaColor clamps a payment-window colour ask to the useful colours
	// only when the window pays a CAST (`cast && e.cast != nil`): an
	// ability's payment (Lathliss's {1}{R} pump) asks the activation's colour
	// unclamped, every colour of the wide production listed, so the spare
	// unit takes the lowest (WUBRGC) and the owed pip its colour -- clamping
	// here would mis-colour the spare (a {W} left as an {R}).
	if clamp && d.class == pdSpell {
		if want, decided := s.windowUseful(k, p, useful); decided && want&mask != 0 {
			mask &= want
		}
	}
	var owed [len(cost.Pips)]uint8
	if !noOwed {
		owed = cost.Pips
		for c := range owed {
			owed[c] -= min(owed[c], s.pl[p].pool[c])
		}
	}
	for i := 0; i < 2; i++ {
		col := bits.TrailingZeros8(mask)
		for c := 0; c < 5; c++ {
			if mask&(1<<c) != 0 && owed[c] > 0 {
				col = c
				owed[c]--
				break
			}
		}
		if !s.floatRestrAb(k, p, sl, ab, col, 1) {
			held[col]++
		}
	}
}

// restrFeeActivateWide activates the wide fee gap sl as gorge's search does:
// the fee paid from the pool, then each pair of colours floated in answer order
// until the plain walk lists play a, else the first answer.
func (s *State) restrFeeActivateWide(k *prep, p uint8, a Action, sl Slot, ab *kop.Ability, fee int) {
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	dc, dn := s.deferSnap(p)
	s.filtFeeSpend(k, p, sl, fee)
	if k.artMana {
		s.artSync(p)
	}
	s.deferSync(p, dc, dn)
	need := s.playNeed(k, a)
	pl := &s.pl[SeatIx(p)]
	rb, pool := s.rb[SeatIx(p)], pl.pool
	var d Decision
	for c := 0; c <= wideAnswers; c++ {
		// the last pass re-floats the first answer when none lists the play
		x, y := wideAnswer(wideEngineAnswer(need, c%wideAnswers))
		s.rb[SeatIx(p)], pl.pool = rb, pool
		if !s.floatRestrAb(k, p, sl, ab, x, 1) || !s.floatRestrAb(k, p, sl, ab, y, 1) {
			s.capFull(StallRestrSrc) // never a silent spill
			return
		}
		if c == wideAnswers {
			return
		}
		s.plainOptions(k, &d, availNoActScript|availNoFeeUnit)
		if hasOption(&d, a) {
			return
		}
	}
}
