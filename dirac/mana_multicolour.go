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
// into pc (prepare), and reads a later bare "{T}: Add one mana of the chosen
// colour" beside a plain fixed first ability as the first's chosen-colour
// twin (Quirion Elves; kop.ChosenSecondManaAb). It leaves pc as the first
// ability alone unless every mana ability of c has one of those shapes.
//
//go:noinline
func prepMultiMana(c *kop.CardDef, pc *pcard) {
	first, nAb := -1, 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			if first < 0 {
				first = j
			}
			nAb++
		}
	}
	if first >= 0 && nAb == 2 {
		for j := first + 1; j < len(c.Abilities); j++ {
			if kop.ChosenSecondManaAb(c, first, j) {
				pc.manaChosen, pc.manaFixedChosen = true, true
				return
			}
		}
	}
	if first >= 0 && nAb == 2 && colourlessThenAny(c, first) {
		// Nightshade Dryad: {T}: Add {C}, then {T}: Add one mana of any
		// colour. gorge's census unit is the {C} alternative then one per
		// colour, Mirrex's gate-on view (mana_gate.go) with no gate: the
		// union C|WUBRG
		pc.manaCol, pc.manaMulti = kop.ColorMask(1)<<kop.C|kop.ColAny, true
		if DebugColourlessAny {
			pc.manaCol, pc.manaMulti = kop.ColorMask(1)<<kop.C, false
		}
		return
	}
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

// DebugColourlessAny is the TestNightshadeDryadMutation switch: the
// {C}-then-any-colour pair keeps its first ability alone ({C}).
var DebugColourlessAny bool

// colourlessThenAny reports card c's two mana abilities, from first, as a
// plain "{T}: Add {C}" then a plain "{T}: Add one mana of any color", both
// unrestricted, amount 1 (kgen colourlessThenAny).
func colourlessThenAny(c *kop.CardDef, first int) bool {
	plain := func(a *kop.Ability) bool {
		return a.AFlags == 0 && a.ManaRestrict == 0 && a.ManaN == 1 && a.Cost.Flags == kop.CostTap &&
			a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && a.Trig.Cond == 0 && !a.ManaEach && len(a.Prog) == 0
	}
	a0 := &c.Abilities[first]
	if !plain(a0) || a0.ManaColors != kop.ColorMask(1)<<kop.C {
		return false
	}
	for j := first + 1; j < len(c.Abilities); j++ {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			return plain(a) && a.ManaColors == kop.ColAny
		}
	}
	return false
}
