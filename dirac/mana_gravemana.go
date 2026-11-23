package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A graveyard mana ability whose {1} fee the floating pool pays (Jack-o'-
// Lantern: "{1}, Exile Jack-o'-Lantern from your graveyard: Add one mana of
// any color", CR 605.1a, 605.2a; kop.CardDef.GraveMana). gorge lists it as a
// payment-window option only while mana floats (the probe
// engine/rules TestGraveManaGorgeProbe), after every battlefield and hand
// source, so the kernel's window takes it where the battlefield and the hand
// run dry (windowTapWalk), and it is an untapped mana source to the same two
// gates the hand guides are (hasUntappedMana, windowUnpricedOpenIf).
//
// Activating it pays the {1} fee from the pool's first non-empty kind in W U B
// R G C order (the fee is unasked: gorge's activation asks no fee pick), moves
// the card to exile (the CARDNAME exile is unasked too) and adds one mana of
// the colour the window's cost still owes. Only a cast's or an activation's
// own payment window lists it, as it lists a hand guide (windowHandMana): the
// unless, ward and trigger-cost windows list the battlefield alone.

// useGraveMana: the table holds a card with a graveyard mana ability.
var useGraveMana = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].GraveMana != 0 {
			return true
		}
	}
	return false
})

// graveManaOn reports whether the table holds a graveyard mana source.
func graveManaOn(k *prep) bool {
	return k.gen != nil && k.gen.uses.has(useGraveMana) && !DebugNoGraveMana
}

// DebugNoGraveMana leaves every graveyard mana source out of the windows and
// gates (mutation testing only: the lockstep must catch it).
var DebugNoGraveMana bool

// graveManaPick is the first card in p's graveyard, in graveyard order, with a
// graveyard mana ability, and the colours it adds. NoObj when none.
func (s *State) graveManaPick(k *prep, p uint8) (o Obj, mask uint8) {
	pl := &s.pl[SeatIx(p)]
	for i := 0; i < int(pl.graveN); i++ {
		if m := k.t.Cards[s.objs[pl.grave[i]].card].GraveMana; m != 0 {
			return pl.grave[i], m
		}
	}
	return NoObj, 0
}

// graveManaAny reports whether p can activate a graveyard mana source: its fee
// needs one floating mana (gorge lists the activation only while one floats).
func (s *State) graveManaAny(k *prep, p uint8) bool {
	if !graveManaOn(k) || poolTotal(&s.pl[SeatIx(p)]) == 0 {
		return false
	}
	o, _ := s.graveManaPick(k, p)
	return o != NoObj
}

// graveManaFee pays the {1} fee from p's pool: the first non-empty kind in W U
// B R G C order.
func (s *State) graveManaFee(p uint8) {
	pl := &s.pl[SeatIx(p)]
	for c := range pl.pool {
		if pl.pool[c] != 0 {
			pl.pool[c]--
			return
		}
	}
}

// graveManaCol is the colour of mask the window's answer takes: the first
// colour the cost still owes after the fee (gorge's askManaColor lists the
// owed colours first, CR 601.2g; windowDemand is that shortfall), else the
// lowest colour of mask. owed is nil for a window whose ask has no clamp
// (anyColor, or none).
func (s *State) graveManaCol(k *prep, p uint8, mask uint8, owed *kop.ManaCost) int {
	mask &= 0x1f
	if owed != nil {
		if m := mask & s.windowDemand(k, p, owed); m != 0 {
			return bits.TrailingZeros8(m)
		}
	}
	return bits.TrailingZeros8(mask)
}

// windowGraveMana is windowTapWalk's pick after the hand's: once neither a
// battlefield source nor a hand guide is listed, the window's answer takes the
// first graveyard source it lists, and reports whether one paid. It is
// withheld, as a hand guide is, when the clamp is decided and no pip colour is
// still owed (windowUseful: nothing it adds pays anything).
func (s *State) windowGraveMana(k *prep, p uint8, useful *kop.ManaCost, clamp, anyColor bool) bool {
	if !s.graveManaAny(k, p) || !s.buildLive() {
		return false
	}
	clamped := clamp && !anyColor
	if clamped {
		if want, decided := s.windowUseful(k, p, useful); decided && want == 0 {
			return false
		}
	} else {
		useful = nil
	}
	o, mask := s.graveManaPick(k, p)
	s.graveManaFee(p)
	s.exileFromGrave(o)
	s.poolAdd(p, s.graveManaCol(k, p, mask, useful), 1)
	return true
}
