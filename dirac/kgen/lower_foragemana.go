package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// forageManaShape reports whether a mana ability's cost is the one the kernel's
// Combo Any search activates (kop.ForageComboAnyAb; dirac mana_forage.go): {T}
// and Forage, nothing else. gorge pays the forage in its mana-activation walk
// (AskManaForage: asked only with two or more options). Whether the production is
// the unrestricted Combo Any one is checked on the built ability
// (forageManaRefusal); any other forage cost on a mana ability stays refused.
func forageManaShape(c lCost) bool {
	if c.Forage == nil || !c.Tap {
		return false
	}
	c.Tap, c.Forage = false, nil
	return c.pureMana() && c.Mana.Generic == 0 && !c.Mana.X && !c.Mana.XX && c.Mana.Pips == [5]int{} &&
		c.Mana.C == 0 && len(c.Mana.Hybrid) == 0 && len(c.Mana.Twobrid) == 0 && len(c.Mana.Phyrexian) == 0
}

// forageManaRefusal names why a lowered mana ability whose cost is {T} and a forage
// is not kop.ForageComboAnyAb: the production or restriction the kernel's Combo Any
// search does not model beside a forage. "" when the ability carries no forage.
func forageManaRefusal(a *kop.Ability) string {
	if a.Kind != kop.AbMana || a.Cost.Flags&kop.CostForage == 0 || kop.ForageComboAnyAb(a) {
		return ""
	}
	return fmt.Sprintf("mana ability cost %s: a forage cost on a mana ability that is not the unrestricted Combo Any production (X mana in any combination of colors) the kernel's combination search pays (mana_forage.go)", kop.ManaCostShape(a))
}

// forageGapMana: a is "{T}, Forage: Add N mana in any combination of colors", unrestricted
// (forageManaRefusal checks the built ability): the one forage shape a mana ability
// beside a plain first one may take.
func forageGapMana(a *lAbility) bool {
	m := a.Mana
	return a.Cost.Forage != nil && forageManaShape(a.Cost) && m != nil && m.Colors == "WUBRG" && m.Choice && !m.Each && m.Special == "" &&
		m.Restrict == nil && m.RestrictAb == nil
}
