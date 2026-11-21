package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Spirit Guide's hand-zone mana ability (Simian / Elvish Spirit Guide:
// "Exile this card from your hand: Add {R}", CR 605.1a, 605.2a;
// kop.CardDef.HandMana). gorge's pay.ManaSourceIDs lists the payer's hand
// after the battlefield, so the guide is:
//
//   - a source of every manual payment window: its "mana" option follows every
//     battlefield source's (one per guide, in hand order) and the window's
//     answer (payexec.WindowAnswer) is the first option listed, so a guide pays
//     only once no battlefield source is listed. A cast's window withholds a
//     guide whose colour the cost no longer absorbs, as it withholds a land
//     (paymentSourceUseful; TestSpiritGuideOffColour). windowTapWalk takes it
//     where windowPick runs dry (handManaPick);
//   - an untapped mana source to every hasUntappedManaSource gate (ward's pay
//     offer, an unless window, the target menu's pool-short candidates):
//     hasUntappedMana and anyManaSource read handManaAny.
//
// gorge's planner and its potential census price no hand source (the probe
// TestSpiritGuideGorgeWindow: no cast is offered that only a guide could pay),
// and the oracle's canonical surface never activates a mana ability at
// priority, so neither does the kernel.

// useHandMana: the table holds a card with a hand mana ability.
var useHandMana = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].HandMana != 0 {
			return true
		}
	}
	return false
})

// handManaOn reports whether the table holds a hand mana source.
func handManaOn(k *prep) bool { return k.gen != nil && k.gen.uses.has(useHandMana) && !DebugNoHandMana }

// handManaPick is the first card in p's hand, in hand order, whose hand mana
// colour the window lists: with the clamp decided (windowUseful), only a
// colour in want. NoObj when none is.
//
//go:noinline
func (s *State) handManaPick(k *prep, p uint8, decided bool, want uint8) (o Obj, col int) {
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.handN); i++ {
		h := pl.hand[i]
		m := k.t.Cards[s.objs[h].card].HandMana
		if m == 0 || decided && m&want == 0 {
			continue
		}
		return h, bits.TrailingZeros8(m)
	}
	return NoObj, 0
}

// handManaAny reports whether p holds a hand mana source (gorge
// untappedManaSource over pay.ManaSourceIDs' hand entries).
func (s *State) handManaAny(k *prep, p uint8) bool {
	if !handManaOn(k) {
		return false
	}
	o, _ := s.handManaPick(k, p, false, 0)
	return o != NoObj
}

// handManaTap activates the guide o: its cost exiles it from the hand (CR
// 118.3, 602.2b) and its mana floats (CR 605.3b).
func (s *State) handManaTap(p uint8, o Obj, col int) {
	s.exileFromGrave(o)
	s.poolAdd(p, col, 1)
}

// windowHandMana is windowTapWalk's last pick: once no battlefield source is
// listed, the window's answer takes the first hand guide it lists. It reports
// whether one paid. Only a cast's or an activation's own payment window lists
// the hand (gorge manaWindowAsk and announcedManaWindowAsk over
// pay.ManaSourceIDs): the unless, ward and trigger-cost windows a resolution
// opens list the battlefield alone (measured: no such window listed a guide
// held in its payer's hand), and none of them runs while a cast is built.
func (s *State) windowHandMana(k *prep, p uint8, useful *kop.ManaCost, clamp, anyColor bool) bool {
	if !handManaOn(k) || !s.buildLive() {
		return false
	}
	want, decided := uint8(0), false
	if clamp && !anyColor {
		want, decided = s.windowUseful(k, p, useful)
	}
	o, col := s.handManaPick(k, p, decided, want)
	if o == NoObj {
		return false
	}
	s.handManaTap(p, o, col)
	return true
}

// DebugNoHandMana leaves every hand guide out of the windows (mutation testing
// only: the lockstep must catch it).
var DebugNoHandMana bool

// handFundDo plays a hand guide's funding step on s (filtDo).
func (s *State) handFundDo(k *prep, p uint8, st filtStep) {
	pl := &s.pl[SeatIx(p)]
	if int(st.hidx) >= int(pl.handN) {
		return
	}
	o := pl.hand[st.hidx]
	if m := k.t.Cards[s.objs[o].card].HandMana; m != 0 {
		s.handManaTap(p, o, bits.TrailingZeros8(m))
	}
}

// handFund is the prefix search's funding pass over the payer's hand: after
// every battlefield source, each hand guide in hand order is one funding
// activation (gorge's scriptSearch.activations walks the raw priority's
// activate options, which list the hand's guides after the battlefield).
func (fs *filtSearch) handFund(k *prep, t *State, funded int, gapUsed bool) bool {
	if DebugNoHandFund || fs.noHand {
		return false
	}
	pl := &t.pl[SeatIx(fs.p)]
	for i := 0; i < int(pl.handN); i++ {
		if k.t.Cards[t.objs[pl.hand[i]].card].HandMana == 0 {
			continue
		}
		fs.hands++
		ok := fs.try(k, t, filtStep{sl: NoSlot, hand: true, hidx: uint8(i)}, funded+1, gapUsed)
		fs.hands--
		if ok {
			return true
		}
	}
	return false
}

// DebugNoHandFund leaves the hand guides out of the prefix search's funding
// pass (mutation testing only: the lockstep must catch it).
var DebugNoHandFund bool
