package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// chosenColorManaShape reports whether sa is the narrow chosen-colour mana
// ability kgen lowers: an AB$ ChooseColor head whose chain reaches a
// Produced$ Chosen DB$ Mana, with a generic-only fee (no {T}) and
// ActivationLimit$ 1. Gorge classifies it a CR 605.1a mana ability
// (cards.IsManaAbilitySA, ManaChainProduction) and resolves the whole chain
// off the stack; the planner-deferred mana-rider route pays the fee and marks
// the once-each-turn use (kop.ManaRiderChosenAnimate).
//
// Only this shape lowers. A {T} head (Nyx Lotus, Nykthos, Rhystic Cave) is the
// colour-asking arm's (lower_askcolourmana.go) when its chain is the devotion
// form, and Rhystic Cave's UnlessCost$ or another rider else keeps the existing
// refusal (lower_dbmanacontext.go).
func chosenColorManaShape(sa *cards.SA) bool {
	if sa == nil || sa.Kind != "AB" || sa.API != "ChooseColor" || !cards.IsManaAbilitySA(sa) {
		return false
	}
	cost := strings.TrimSpace(sa.Params["Cost"])
	n, err := strconv.Atoi(cost)
	if err != nil || n < 1 {
		return false // a {T}, pip, life or sacrifice cost is another shape
	}
	if strings.TrimSpace(sa.Params["ActivationLimit"]) != "1" {
		return false
	}
	man := sa.Sub
	if man == nil || man.API != "Mana" || strings.TrimSpace(man.Params["Produced"]) != "Chosen" {
		return false
	}
	if amt := strings.TrimSpace(man.Params["Amount"]); amt != "" && amt != "1" {
		return false
	}
	if mc := strings.TrimSpace(man.Params["Cost"]); mc != "" && mc != cost {
		return false
	}
	if strings.TrimSpace(man.Params["UnlessCost"]) != "" || strings.TrimSpace(man.Params["RestrictValid"]) != "" ||
		strings.TrimSpace(man.Params["PersistentMana"]) != "" || strings.TrimSpace(man.Params["PersistentUntilEndOfCombat"]) != "" {
		return false
	}
	sub := man.Sub
	if sub == nil {
		return true
	}
	if sub.API != "Animate" || !strings.EqualFold(strings.TrimSpace(sub.Params["Colors"]), "ChosenColor") ||
		strings.TrimSpace(sub.Params["OverwriteColors"]) != "True" {
		return false
	}
	switch d := strings.TrimSpace(sub.Params["Duration"]); d {
	case "", "EOT":
	default:
		return false
	}
	if strings.TrimSpace(sub.Params["ValidTgts"]) != "" || strings.TrimSpace(sub.Params["TargetType"]) != "" {
		return false
	}
	if sub.Sub == nil || sub.Sub.API != "Cleanup" || sub.Sub.Sub != nil || strings.TrimSpace(sub.Sub.Params["ClearChosenColor"]) != "True" {
		return false
	}
	return true
}

// lowerChooseColorMana lowers the chosen-colour mana ability chosenColorManaShape
// admits (Foraging Wickermaw). The head's {1} is the ability's one cost; the
// Mana node's own Cost$ 1 is the same Forge part (gorge charges the head's
// cost only), so it is checked against it and never added. The Animate rider
// lowers into the ability's Prog as the existing chosen-colour Override (five
// WUBRG records, kop.OvChosenColor); the planner-deferred rider route runs it
// after the mana, with the activation's colour answer recorded.
func (lc *lowerCtx) lowerChooseColorMana(sa *cards.SA, ps *params, ab *lAbility) error {
	if ab.Index >= 8 {
		// perm.actUsed is a uint8, so the once-each-turn bit for ability 8 or
		// later would silently never record or block (the same bound
		// mana_filterfirst.go's prepFiltFirst panics on). The shape is not
		// tied to Wickermaw's name, so refuse it by index.
		return refuse("ChooseColor mana ability at index %d: the kernel's once-each-turn bit is a uint8", ab.Index)
	}
	if d, ok := ps.get("Defined"); ok && strings.TrimSpace(d) != "You" {
		return refuse("ChooseColor Defined$ %q", d)
	}
	if v, ok := ps.get("InstantSpeed"); ok && strings.TrimSpace(v) != "" {
		return refuse("ChooseColor InstantSpeed$ %q", v)
	}
	ps.consume("SubAbility")
	man := sa.Sub
	mp := newParams(man.Params)
	if prod := mp.str("Produced"); prod != "Chosen" {
		return refuse("ChooseColor Mana Produced$ %q", prod)
	}
	mp.consume("Produced")
	if v, ok := mp.get("Amount"); ok && strings.TrimSpace(v) != "1" {
		return refuse("ChooseColor Mana Amount$ %q", v)
	}
	if v, ok := mp.get("Cost"); ok && strings.TrimSpace(v) != strings.TrimSpace(sa.Params["Cost"]) {
		return refuse("ChooseColor Mana Cost$ %q beside the head's Cost$ %q", v, sa.Params["Cost"])
	}
	for _, k := range [...]string{"UnlessCost", "RestrictValid", "PersistentMana", "PersistentUntilEndOfCombat", "AddsKeywords", "AddsCounters", "Defined"} {
		if v, ok := mp.get(k); ok {
			return refuse("ChooseColor Mana %s$ %q", k, v)
		}
	}
	mp.consume("SubAbility")
	if left := mp.unread(); len(left) > 0 {
		return refuse("ChooseColor Mana unread params %v", left)
	}
	m := &lMana{Colors: "WUBRG", Amount: cst(1), Rider: riderChosenAnimate}
	if man.Sub != nil {
		// the rider is the same chosen-colour Animate the mid-resolution
		// ChooseColor path lowers (lower_effect_damage.go's colGrant)
		lc.colGrant = true
		err := lc.chain(man.Sub, nil)
		lc.colGrant = false
		if err != nil {
			return err
		}
		if len(ab.Prog) != 1 || ab.Prog[0].Op != "Override" || !strings.Contains(ab.Prog[0].CS, "chosenColor") {
			return refuse("ChooseColor Mana rider %v: not the chosen-colour animate", ab.Prog)
		}
	}
	ab.Mana = m
	if left := ps.unread(); len(left) > 0 {
		return refuse("ChooseColor unread params %v", left)
	}
	return nil
}
