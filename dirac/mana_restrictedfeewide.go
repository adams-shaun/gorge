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
	fee, ok := s.windowFeeWideCost(k, p, sl)
	return ok && s.windowPoolPays(p, &fee, false)
}

// windowFeeWideCost is windowFeeWide's gap test without the pool: the fee of
// p's untapped wide restricted fee gap sl, and whether sl is one.
func (s *State) windowFeeWideCost(k *prep, p uint8, sl Slot) (fee kop.ManaCost, ok bool) {
	if k.gen == nil || !k.gen.anyRestrSrc || !s.restrUsable(k, sl) {
		return fee, false
	}
	d := s.payDescNow(NoObj)
	v, kind, ab := s.restrView(k, p, sl, s.manaPCGen(k, sl), &d)
	if v == nil || kind != rkExt || v.restrFeeCol != 0 || restrFeeEmpty(v) || !restrFeeWideAb(ab) {
		return fee, false
	}
	return restrFeeCost(v), true
}

// windowFeeFund is the autopay window's pick for a wide restricted fee gap
// the pool cannot pay yet and no listed source is useful for: gorge's script
// search taps the first covered source in battlefield order to fund the gap's
// fee whatever colour it makes (potentialPrefixScript's funding; the offer
// priced it, restrSrcAvailD's fee pass), where the window's clamp keeps only
// the colours the cost still owes. A Sol Ring's {C}{C} funds an Orb of
// Dragonkind's {1} for a {R} pump (the Orb's {R} floats restricted).
func (s *State) windowFeeFund(k *prep, p uint8, order []Slot, decided, census, potential bool) Slot {
	if !decided || census || potential || k.gen == nil || !k.gen.anyRestrSrc || s.cfg&cfgGorgeAutopay == 0 {
		return NoSlot
	}
	for _, g := range order {
		fee, ok := s.windowFeeWideCost(k, p, g)
		if !ok || s.windowPoolPays(p, &fee, false) {
			continue
		}
		if sl, _, ok := s.restrFeeFundSrc(k, p, g); ok && s.windowSourceUsable(k, sl, p) {
			return sl
		}
		return NoSlot
	}
	return NoSlot
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
	clampUndecided := false
	if clamp && d.class == pdSpell {
		want, decided := s.windowUseful(k, p, useful)
		if decided && want&mask != 0 {
			mask &= want
		} else if !decided {
			clampUndecided = true
		}
	}
	// The window's own pips the floating pool cannot cover are owed whatever
	// the ask carries (gorge's tape answers the plan's production, whose
	// colours serve the payment, not the ask's cost field): the owed pips
	// take their colour first, then the ask's first listed colour -- an
	// undecided cast clamp lists the owed colours first (askManaColor's
	// owedColoursFirst), so the spare takes an owed colour there too.
	var owed, owedSet [len(cost.Pips)]uint8
	owed = cost.Pips
	for c := range owed {
		owedSet[c] = min(owed[c], 1) - min(owed[c], s.pl[p].pool[c])
		owed[c] -= min(owed[c], s.pl[p].pool[c])
	}
	var owedMask uint8
	for c := range owedSet {
		owedMask |= uint8(owedSet[c]) << c
	}
	for i := 0; i < 2; i++ {
		pick := mask
		if clampUndecided && owedMask&mask != 0 {
			pick = owedMask & mask
		}
		col := bits.TrailingZeros8(pick)
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
