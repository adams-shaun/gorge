package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Several fixed-colour mana abilities on one permanent (kmana2): a dual
// land's or a triome's intrinsic "{T}: Add {c}" abilities (CR 305.6), each
// a plain tap ability of one colour. gorge's planner census lists such a
// source once, at its battlefield position among the deterministic units,
// with one alternative per ability (rules windowManaUnitsWith); its rank
// reads the source's colours and the number of types it can make (Alt.Flex,
// Alt.Colours), and the witness names the colour (Produces; the intrinsic
// abilities share one identity). The kernel's unit is that source: the
// union of the colours (pcard.manaCol), placed with the fixed-production
// units (plan's pass 0), not after them as a choice-shaped one
// (manaMulti). The payment window activates its first ability (windowTap:
// the lowest colour, or a gated Verge's ability 0, mana_gate.go). kgen accepts only this shape (refuseMultiMana:
// abilities in W U B R G C order, colours distinct).

// prepMultiMana folds card c's further plain fixed-colour mana abilities
// into pc (prepare). It leaves pc as the first ability alone unless every
// mana ability of c has that shape.
//
//go:noinline
func prepMultiMana(c *kop.CardDef, pc *pcard) {
	var col kop.ColorMask
	n := 0
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if a.AFlags != 0 || a.ManaRestrict != 0 || a.ManaN > 1 || a.Cost.Flags != kop.CostTap || a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 ||
			bits.OnesCount8(uint8(a.ManaColors&0x3f)) != 1 || col&a.ManaColors != 0 {
			return
		}
		col |= a.ManaColors
		n++
	}
	if n > 1 {
		pc.manaCol, pc.manaMulti = col, true
	}
}
