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
