package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// exileGraveManaShape reports whether a mana ability's cost is the one the
// kernel's tap-another search activates (kop.ExileGManaAb; dirac
// mana_exilegrave.go): {T} and ExileFromGrave<1/Spec>, nothing
// else. gorge pays the exile in its mana-activation walk (one "mana_exile" pick
// over the payer's graveyard, asked even with one candidate); the CARDNAME
// singleton, the graveyard-or-battlefield and hand forms, the announced X and the
// forage either-or are other cost shapes and stay refused.
func exileGraveManaShape(c lCost) bool {
	if c.ExileG == nil || !c.Tap || c.ExileGSelf || c.exileGCard || c.ExileHand || c.ExileGBf || c.ExileGX || c.ExileGN != 1 {
		return false
	}
	c.Tap, c.ExileG, c.ExileGN = false, nil, 0
	return c.pureMana() && c.Mana.Generic == 0 && !c.Mana.X && !c.Mana.XX && c.Mana.Pips == [5]int{} &&
		c.Mana.C == 0 && len(c.Mana.Hybrid) == 0 && len(c.Mana.Twobrid) == 0 && len(c.Mana.Phyrexian) == 0
}

// exileGraveManaRefused: a mana ability's lowered cost carries a forage part or an
// exile-from-graveyard part outside the modelled shape (lower.go's refusal).
func exileGraveManaRefused(c lCost) bool {
	return c.Forage != nil && !forageManaShape(c) || c.ExileG != nil && !exileGraveManaShape(c)
}

// exileGraveManaCostRefused is build_ab.go's twin: the built cost holds a forage,
// a return or an exile-from-graveyard part outside the modelled shape.
func exileGraveManaCostRefused(c kop.Cost) bool {
	return c.Flags&kop.CostReturn != 0 || c.Flags&kop.CostForage != 0 && !kop.ForageManaCost(&c) || c.Flags&kop.CostExileGrave != 0 && !exileGraveManaCost(c)
}

// exileGraveManaWhy is the refusal text of a mana ability whose cost carries a
// forage or an exile-from-graveyard part exileGraveManaShape does not admit: a
// rider, a graveyard activation zone or a spend restriction is named, so the
// census says which mechanism the card waits on.
func exileGraveManaWhy(c lCost, ps *params) string {
	const base = "mana ability with a forage / exile-from-graveyard cost"
	if c.Forage != nil {
		return base
	}
	switch {
	case (ps.str("TriggersWhenSpent") != ""):
		return base + " and a TriggersWhenSpent$ rider"
	case graveManaFee(c, ps):
		return base + ": graveyard mana ability with a mana fee (ActivationZone$ Graveyard, a mana cost beside the CARDNAME exile)"
	case (ps.str("ActivationZone") != "") || c.ExileGSelf || c.exileGCard:
		return base + " on an ability activated from the graveyard (ActivationZone$ / CARDNAME exile)"
	case (ps.str("RestrictValid") != ""):
		return base + " and a spend restriction (RestrictValid$)"
	case (ps.str("SubAbility") != ""):
		return base + " and a SubAbility$ rider"
	}
	return base
}

// graveManaFee: a mana ability activated from the graveyard whose cost is a
// mana fee beside the CARDNAME exile (Jack-o'-Lantern's {1}). gorge lists it as
// a priority activation only while a mana floats, pays the fee from the
// floating pool (colours in W U B R G order) and asks a colour; the kernel has
// no graveyard source with a fee the pool pays (gravemana_probe_test.go in
// engine/rules records the option lists).
func graveManaFee(c lCost, ps *params) bool {
	m := c.Mana
	fee := m.Generic != 0 || m.X || m.XX || m.Pips != [5]int{} || m.C != 0 || len(m.Hybrid) != 0 || len(m.Twobrid) != 0 || len(m.Phyrexian) != 0
	return fee && ps.str("ActivationZone") == "Graveyard" && c.exileGCard
}

// exileGraveManaCost reports whether the lowered cost of a mana ability is
// exactly {T} plus an exile-from-graveyard part (build_ab.go's twin of
// exileGraveManaShape).
func exileGraveManaCost(c kop.Cost) bool {
	return c.Flags == kop.CostTap|kop.CostExileGrave && !c.ExileHand && !c.ExileGSelf && !c.ExileBf && !c.Exert && !c.DiscardHand &&
		c.N == 1 && c.Mana == (kop.ManaCost{})
}

// exileGraveManaRefusal names why a lowered mana ability whose cost carries an
// exile-from-graveyard part is not kop.ExileGManaAb: the restriction, rider,
// activation zone or other cost part the kernel's search does not model. "" when
// the ability is not one of them.
func exileGraveManaRefusal(a *kop.Ability) string {
	if a.Kind != kop.AbMana || a.Cost.Flags&kop.CostExileGrave == 0 {
		return ""
	}
	why := ""
	switch {
	case a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield:
		why = "an activation zone other than the battlefield (the graveyard-activated mana ability)"
	case a.ManaRestrict != 0 || a.ManaRestrictAb != 0:
		why = "a spend restriction on the mana (RestrictValid$): the exile-from-graveyard source is not composed with the restricted-mana search"
	case a.ManaRider != kop.ManaRiderNone || len(a.Prog) != 0:
		why = "a rider on the mana ability (SubAbility$): the exile-from-graveyard source is not composed with the mana-rider search"
	case a.Trig.Cond != 0 || a.Gate.Ctr != 0:
		why = "a TriggersWhenSpent$ rider or a gate on the mana ability"
	case a.Cost.Flags&^(kop.CostTap|kop.CostExileGrave) != 0 || a.Cost.Mana != (kop.ManaCost{}) || a.Cost.ExileHand || a.Cost.ExileGSelf || a.Cost.ExileBf:
		why = "an exile-from-graveyard part beside another cost part, or a CARDNAME / hand / battlefield exile form"
	case a.Cost.Flags&kop.CostTap == 0:
		why = "no {T} in the cost"
	default:
		why = "an ability shape the exile-from-graveyard search does not model"
	}
	return fmt.Sprintf("mana ability cost %s: exile-from-graveyard mana source with %s", kop.ManaCostShape(a), why)
}
