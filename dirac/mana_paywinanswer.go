package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// payWindow's answers (mana.go): gorge poses an unless or ward window's mana
// abilities with no ManaPayment and no pending cast, so neither ask is
// reordered by what the cost owes (owedAbilitiesFirst needs a cast) and
// payexec.WindowAnswer takes the lowest W U B R G C symbol listed, else the
// first option. A walk is payWindow's (or its ward fork's) when it is noOwed
// with no clamp, no any-colour rider and no potential pricing.

// walkColNoOwed is the noOwed windowWalkCol reads for windowTapWalk's plain tap:
// the walk's own noOwed, and also the colour ask of a one-ability source in
// xWindowPay's walk (payAsk). gorge pays an announced-X activation through its
// ordinary window (manaWindowAsk): neither the source list nor the colour ask
// that follows an activation carries a ManaPayment, and with no announced
// hybrid pip and no fallen-back plan (rules.PendingCastOwed is not ok for an
// activation) the oracle's answer owes nothing (windowOwed) and takes the
// lowest colour listed (payexec.WindowAnswer): Arcane Signet under a Naya
// commander answers {W} for Kessig Wolf Run's {X}{R}{G} with the generic still
// unpaid. Once the generic is paid the clamp lists only the owed pips, which
// windowWalkCol's noOwed clamp arm answers. A multi-ability source's ability
// ask keeps the owed-first answer (windowFiltOwedPick).
func (s *State) walkColNoOwed(noOwed, payAsk bool, pc *pcard) bool {
	return noOwed || payAsk && !pc.manaMulti && s.bld().pipN == 0
}

// windowWalkCol is the colour windowTapWalk's plain tap of sl answers.
func (s *State) windowWalkCol(k *prep, p uint8, cost, useful *kop.ManaCost, sl Slot, pc *pcard, clamp, anyColor, noOwed, potential bool) int {
	var owed kop.ManaCost
	col := s.windowColOwed(k, p, cost, s.windowOwedCost(k, p, cost, &owed), sl, pc, anyColor, noOwed)
	if noOwed && clamp && !anyColor {
		if pc.manaMulti {
			// a cast's ability ask lists what the cost owes first (gorge owedAbilitiesFirst)
			col = s.windowAbilityAskCol(k, p, useful, sl, pc)
		} else if want, decided := s.windowUseful(k, p, useful); decided {
			// the colour ask lists only what the clamp keeps (gorge
			// paymentUsefulColours): with the generic covered, an
			// any-colour ability answers the owed pip
			if c, ok := s.painAnyOwed(p, useful, pc); ok {
				col = c
			} else if m := s.windowSrcMask(k, sl) & want; m != 0 {
				col = bits.TrailingZeros8(m)
			}
		} else if c, ok := s.windowGrantFirstCol(k, p, sl, useful); ok {
			col = c // the unsettled generic keeps every colour: the grant that makes the owed pip is listed first (mana_grantfirst.go)
		}
	}
	if noOwed && !clamp && !anyColor && !potential && payWinPainAnyC(k, pc) {
		col = kop.C
	}
	return col
}

// payWinPainAnyC: sl's painland ask in payWindow takes its plain {C}. A
// Produced$ Any painful ability beside a {C} one (Grand Coliseum, Starting
// Town) is listed "Add C", "Add any color": neither carries a symbol, so the
// answer is the first, {C}, with no pain, whatever colour the cost needs.
func payWinPainAnyC(k *prep, pc *pcard) bool {
	return k.painMana && pc.painCol == kop.ColAny && !pc.painOnly && uint8(pc.manaCol)&0x1f == 0
}

// payWinHybFilt activates the hybrid-fee filter at sl (Graven Cairns' "{B/R},
// {T}: Add {B}{B}, {B}{R}, or {R}{R}", mana_filterhybrid.go) when payWin and
// the pool pays its fee: gorge lists "Add C", "Add BB", "Add RR" and the
// answer takes the lowest symbol, the filter's lowest colour, never the
// symbol-less {C}. An unpaid fee lists only {C}, the plain tap.
func (s *State) payWinHybFilt(k *prep, p uint8, sl Slot, payWin bool) bool {
	if !payWin || k.gen == nil || s.filtFirstOnly(k, sl) {
		return false
	}
	if _, _, ok := s.filtHyb(k, sl); !ok || !s.filtUsable(k, sl) || !s.filtPaid(k, p, sl) {
		return false
	}
	s.windowFiltFirst(k, p, sl, nil, true)
	return true
}

// windowHybFiltArm is windowTapWalk's hybrid-fee filter answer: payWindow's
// (payWinHybFilt), or a madness cast's window, which lists the owed-colour
// filter ahead of the plain tap (owedAbilitiesFirst reads the cast's
// composition) but answers the lowest symbol the clamp keeps
// (windowFiltFirstFallback): Graven Cairns floats {B}{B} beside a floating {B}
// for a {1}{R}{R} cost it cannot pay, and the cast is reversed (CR 733.1,
// kw_madness.go).
func (s *State) windowHybFiltArm(k *prep, p uint8, sl Slot, useful *kop.ManaCost, noOwed, clamp, anyColor, potential bool) bool {
	if s.payWinHybFilt(k, p, sl, noOwed && !clamp && !anyColor && !potential) {
		return true
	}
	if !clamp || anyColor {
		return false
	}
	if !s.madnessWindowLive() || !s.windowFiltOwedPick(k, p, sl, useful) {
		return noOwed && !potential && s.windowHybFiltFallback(k, p, sl, useful)
	}
	s.windowFiltFirstFallback(k, p, sl, useful)
	return true
}

// windowHybFiltFallback is a cast's fallback window (a PaymentFallback with no
// ManaPayment: convoke, Spree, a lowered or free cast) at a hybrid-fee filter
// beside its plain {C} (Wooded Bastion's "{G/W}, {T}: Add {G}{G}, {G}{W} or
// {W}{W}") whose fee the floating mana pays. The source's ask lists "Add C",
// "Add GG", "Add WW" and payexec.WindowAnswer, with nothing owed, takes the
// lowest symbol listed, never the symbol-less {C}: the filter's lowest colour
// the clamp keeps (windowFiltFirstFallback). A source the clamp does not offer
// never reaches here (windowClampSkip), and an unpaid fee lists only {C}.
func (s *State) windowHybFiltFallback(k *prep, p uint8, sl Slot, useful *kop.ManaCost) bool {
	if k.gen == nil || s.filtFirstOnly(k, sl) {
		return false
	}
	if _, _, ok := s.filtHyb(k, sl); !ok || !s.filtUsable(k, sl) || !s.filtPaid(k, p, sl) {
		return false
	}
	s.windowFiltFirstFallback(k, p, sl, useful)
	return true
}

// DebugWindowColourPlanner keeps pickUnitColour's Produces preference for the
// plans a payment gorge takes through its manual mana window (mutation
// testing only: TestWindowColourGenericOnlyMutation must see the divergence
// the window's answer prevents). It also skips blgSubsumedFirst, which names
// the same colour for a Combo land under a granted basic type and so covers
// the repro without the window's answer.
var DebugWindowColourPlanner bool

// planWindowCol is the colour a plan unit at sl makes when the payment is one
// gorge takes through its manual mana window (an activation whose sacrifice
// part follows the mana): a printed source that asks a choice of colours (a
// Combo) answers an owed colour first in its ask order, else the window's
// fallback for the source (windowFallbackCol, a granted basic land type's
// colour first, mana_window.go). The ask's first token only orders the owed
// colours: with nothing owed, payexec.WindowAnswer takes the lowest W U B R G
// C symbol the ask lists, which for a granted-type source is the granted
// type's own colour (windowColOwed's blgFirst answer). The ask order is the
// one windowCol reads (colourAskOrder: the ability's printed token order,
// else the colours of the mask in W U B R G C order).
//
// Only a source that itself gains a basic land type takes this answer. The
// grant adds an intrinsic ability the window lists beside the source's own,
// and gorGe's payment for such a source is the one the two staged repros
// record (Luxury Suite and Foul Orchard under Urborg, Tomb of Yawgmoth). A
// source without a grant keeps the planner's answer (ok false): its lockstep
// repros record the planner's colour (tail-01 The Mana Rig: Skybridge Towers
// floats the planner's U, not the ask's W), so overriding it would regress
// them. A fixed or any-colour ability has no named choice either.
func (s *State) planWindowCol(k *prep, p uint8, cost *kop.ManaCost, sl Slot) (int, bool) {
	if k.gen == nil || s.blgMask(k, sl) == 0 {
		return 0, false // the source itself gains no basic land type
	}
	pc := s.manaPCAt(k, sl)
	cd := s.cardAt(sl)
	if pc.manaAb < 0 || int(pc.manaAb) >= len(k.t.Cards[cd].Abilities) {
		return 0, false // no own ability to read an ask order from
	}
	ab := &k.t.Cards[cd].Abilities[pc.manaAb]
	// A granted ability's manaAb indexes the granting card, not this one, and a
	// granted basic land type's view adds colours of its own: only the source's
	// own printed ability has the ask order the window reads.
	if ab.Kind != kop.AbMana || uint8(ab.ManaColors)&0x3f != uint8(pc.manaCol)&0x3f {
		return 0, false
	}
	m := uint8(ab.ManaColors) & 0x1f
	if m == 0 || m == uint8(kop.ColAny) || m&(m-1) == 0 {
		return 0, false // a fixed production, or any colour: no named choice
	}
	var ord [kop.NumManaKinds]uint8
	n, printed := colourAskOrder(ab, m, &ord)
	if !printed || n == 0 {
		// no recorded token order: the colours of m in W U B R G C order
		// (colourAskOrder's common case), so an owed pip above the lowest is
		// still named before the fallback
		n = 0
		for c := 0; c < 5; c++ {
			if m&(1<<uint(c)) != 0 {
				ord[n] = uint8(c)
				n++
			}
		}
	}
	var owed kop.ManaCost
	s.windowOwedCost(k, p, cost, &owed)
	for i := 0; i < n; i++ {
		if c := int(ord[i]); c < 5 && owed.Pips[c] > s.pl[p].pool[c] {
			return c, true // an owed pip comes first in the ask order
		}
	}
	for c := 0; c < 5; c++ {
		if owed.Pips[c] > s.pl[p].pool[c] {
			return 0, false // an owed colour the ask order does not name: the planner keeps coverage
		}
	}
	// nothing owed: the window's answer for this source (windowColOwed's
	// fallback, mana_window.go): the lowest colour of the printed mask, a
	// granted basic land type's colour first — not the ask's first token
	col := windowFallbackCol(pc, m)
	if pc.blgFirst != 0 {
		col = int(pc.blgFirst - 1)
	}
	return col, true
}
