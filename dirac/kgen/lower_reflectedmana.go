package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// AB$ ManaReflected in its one planned colour-read shape over the
// controller's own permanents ("add one mana of any color among legendary
// permanents you control": Plaza of Heroes). gorge pay.PaymentPlanReflectedShape
// admits ReflectProperty$ Is over a Valid$ whose every alternative carries
// YouCtrl; the census resolves the colours with the resolution's own walk
// (effects.ManaReflectedObjects) and drops {C} (PaymentPlanReflectedCounts).
// The kernel reads exactly: Cost$ T, ColorOrType$ Color, ReflectProperty$ Is,
// Valid$ <filter>[,<filter>] each with YouCtrl. Anything else is refused.
func (lc *lowerCtx) lowerManaReflected(sa *cards.SA, ps *params, ab *lAbility) error {
	if ps.str("ColorOrType") != "Color" {
		return refuse("ManaReflected ColorOrType$ %q", ps.str("ColorOrType"))
	}
	if ps.str("ReflectProperty") != "Is" {
		return refuse("ManaReflected ReflectProperty$ %q", ps.str("ReflectProperty"))
	}
	valid := ps.str("Valid")
	if valid == "Defined.Self" && lc.granted > 0 {
		// (Katilda, Dawnhart Prime) a GRANTED "{T}: Add one mana of any
		// of this creature's colors" reads the grantee's own colours
		// (pay/planreflected.go reflectsStableObjects); the
		// kernel's granted source takes the colours of the slot it is
		// granted to (kernel mana_owncolours.go).
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "ReflectedSelf"}
		return lc.finishMana(sa, ps, ab, m)
	}
	if valid == "Defined.Imprinted" && lc.imprintMox && lc.granted == 0 {
		// (Chrome Mox) the colours of the card the enters trigger exiled,
		// read through its reverse exiled-with record (lower_imprintmox.go;
		// kernel mana_imprint.go). The card records what it exiles and
		// its text names ExiledWithSource, so the record is the reverse one
		m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "ReflectedImprint"}
		lc.out.ExiledWith, lc.out.ExwWord = true, true
		return lc.finishMana(sa, ps, ab, m)
	}
	if valid == "" || strings.HasPrefix(valid, "Defined.") {
		return refuse("ManaReflected Valid$ %q", valid)
	}
	for _, alt := range strings.Split(valid, ",") {
		if !strings.Contains(alt, "+YouCtrl") {
			// gorge's shape needs the controller read on every alternative
			// (reflectsStableObjects); OppCtrl reads lands' production
			return refuse("ManaReflected Valid$ %q without YouCtrl", valid)
		}
	}
	f, err := lc.filter(valid)
	if err != nil {
		return err
	}
	m := &lMana{Colors: "WUBRG", Choice: true, Amount: cst(1), Special: "Reflected", SpecialF: f}
	return lc.finishMana(sa, ps, ab, m)
}

// reflTapMana: a is the lowered reflected ability, cost {T} alone.
func reflTapMana(a *lAbility) bool {
	if a.Mana == nil || a.Mana.Special != "Reflected" && a.Mana.Special != "ReflectedImprint" {
		return false
	}
	c := &a.Cost
	m := &c.Mana
	if !c.Tap || c.SacSelf || c.Sac != nil || c.DiscardN != 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr != 0 || c.PayLife != 0 ||
		c.TapN != 0 || c.TapHost || c.SacHost || c.Forage != nil || c.ExileG != nil || c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Return != nil {
		return false
	}
	if m.Pips != [5]int{} || len(m.Hybrid) > 0 || m.X || m.C != 0 || m.Generic != 0 {
		return false
	}
	return len(a.Prog) == 0 && a.ActCond.Kind == cNone && a.Limit == 0 && !a.TurnOnce && a.Gate == nil && !a.Sorcery && !a.Loyalty
}

// refuseReflectedMana handles a card with a reflected mana ability. The
// kernel models exactly Plaza of Heroes' three: {T}: Add {C}; {T}: Add one
// mana of any colour, restricted (a Courtyard-like census gap, genPrep.anyR);
// {T}: the reflected colour. gorge's planner census lists one unit: the {C}
// alternative, the reflected colours (normal tier), and the restricted any
// (deferred); the kernel's view is the union {C} | the reflected colours
// (kernel mana_reflected.go). It reports whether ms holds a
// reflected ability (the caller then has nothing to add).
func (lc *lowerCtx) refuseReflectedMana(ms []*lAbility) bool {
	for _, a := range ms {
		if a.Mana.Special != "ReflectedImprint" {
			continue
		}
		// the imprint mox's one ability (lower_imprintmox.go): the kernel
		// models it alone, {T} its whole cost
		if len(ms) != 1 || !reflTapMana(a) {
			lc.refuseNode("card", "imprint-reflected mana ability beside another mana ability or with a cost beyond {T}")
		}
		return true
	}
	at := -1
	for i, a := range ms {
		if a.Mana.Special == "Reflected" {
			if at >= 0 {
				lc.refuseNode("card", "two reflected mana abilities")
				return true
			}
			at = i
		}
	}
	if at < 0 {
		return false
	}
	switch {
	case len(ms) != 3 || at != 2:
		lc.refuseNode("card", "reflected mana ability outside {C}, restricted any colour, reflected")
	case !plainTapMana(ms[0], false) || !unrestrictedOne(ms[0]) || ms[0].Mana.Colors != "C" || ms[0].Mana.Choice:
		lc.refuseNode("card", "reflected mana ability beside a first ability other than {T}: Add {C}")
	case !plainTapMana(ms[1], false) || ms[1].Mana.Colors != "WUBRG" || ms[1].Mana.Restrict == nil || ms[1].Mana.RestrictAb != nil ||
		ms[1].Mana.Amount.Kind != vConst || ms[1].Mana.Amount.N != 1:
		lc.refuseNode("card", "reflected mana ability beside a second ability other than a restricted any colour")
	case !reflTapMana(ms[2]):
		lc.refuseNode("card", "reflected mana ability with a cost beyond {T}")
	}
	return true
}
