package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// tapExtraStatic lowers a `Mode$ TapsForMana | Static$ True` trigger to the
// StatTapExtra static when gorge's planner folds it into a tap's production
// (pay.PaymentPlanFoldableTapMana: one literal symbol and amount, no other
// parameter), or a reflected body over lands (Lavaleaper, Mana Flare:
// pay.PaymentPlanReflectedTapMana, one mana of the type the tap produced).
// Anything else keeps refusing: an
// any-colour body is a deferred source in gorge, planned by hand in the mana
// window.
func (lc *lowerCtx) tapExtraStatic(t cards.Trigger) error {
	tm, ok := pay.PaymentPlanFoldableTapMana(lc.face, t)
	if !ok {
		return lc.tapExtraReflected(t)
	}
	if tm.ToTapped {
		return refuse("TapsForMana adding mana to the tapped permanent's controller")
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	if v := strings.TrimSpace(tp.str("Activator")); v != "You" {
		return refuse("TapsForMana Activator$ %q", v)
	}
	tp.consume("Activator")
	st := lStatic{Kind: "TapExtra"}
	var err error
	if v := tp.str("ValidCard"); v != "" {
		if st.F, err = lc.filter(v); err != nil {
			return err
		}
		tp.consume("ValidCard")
	}
	if st.F == nil {
		return refuse("TapsForMana without ValidCard$")
	}
	switch v := strings.TrimSpace(tp.str("Produced")); v {
	case "":
	case "C":
		st.Flags = append(st.Flags, "TapOnC")
		tp.consume("Produced")
	default:
		return refuse("TapsForMana Produced$ %q", v)
	}
	n := 0
	for i, c := range tm.Mana {
		if c > 0 {
			n++
			st.TapCol, st.TapAmt = uint8(i), uint8(c)
		}
	}
	if n != 1 {
		return refuse("TapsForMana adding several colours")
	}
	lc.out.Statics = append(lc.out.Statics, st)
	return nil
}

// tapReflectLandFilters are the ValidCard$ filters of a reflected TapsForMana
// body that reach lands only.
var tapReflectLandFilters = cards.NewNameSet("Land.Basic", "Land", "Mountain,Forest,Plains")

// tapExtraReflected lowers a reflected TapsForMana body (gorge
// pay.PaymentPlanReflectedTapMana) to a StTapReflect StatTapExtra: one mana
// of the type the tap produced. A land filter lowers (tapReflectLandFilters):
// gorge folds the extra per production (pay.PaymentPlanReflectedMana over each
// single-colour alternative of a choice source), so the kernel prices it on
// the pick; a source making several types in one tap stays the kernel's
// StallTapExtra. ValidCard$ Artifact.token (Roxanne, Starfall Savant) reaches a
// source that makes any colour (Meteorite): the planner reads it as one tap of
// one more mana of the colour picked (dirac mana_tapextra.go); a token whose
// extra it cannot price that way (a Treasure) stalls there, as every other
// shape. Any other filter refuses. The extra goes to the tapping
// player: either the controller's own taps (Activator$ You, Defined$ You) or
// every player's (no Activator$, Defined$ TriggeredActivator or
// TriggeredCardController, the payer of a planned tap), StTapAnyPlayer.
func (lc *lowerCtx) tapExtraReflected(t cards.Trigger) error {
	tm, ok := pay.PaymentPlanReflectedTapMana(lc.face, t)
	if !ok {
		return refuse("trigger mode TapsForMana")
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static", "TriggerZones", "TriggerDescription")
	st := lStatic{Kind: "TapExtra", TapAmt: 1, Flags: []string{"TapReflect"}}
	switch v := strings.TrimSpace(tp.str("Activator")); {
	case v == "You" && !tm.ToTapped:
	case v == "" && tm.ToTapped:
		st.Flags = append(st.Flags, "TapAnyPlayer")
	default:
		return refuse("TapsForMana reflected mana with Activator$ %q", v)
	}
	tp.consume("Activator")
	if v := strings.TrimSpace(tp.str("ValidCard")); !tapReflectLandFilters.Has(v) && v != "Artifact.token" {
		return refuse("TapsForMana reflected mana over ValidCard$ %q: not a land filter", v)
	}
	var err error
	if st.F, err = lc.filter(tp.str("ValidCard")); err != nil {
		return err
	}
	tp.consume("ValidCard")
	if v := strings.TrimSpace(tp.str("Produced")); v != "" {
		return refuse("TapsForMana reflected mana with Produced$ %q", v)
	}
	lc.out.Statics = append(lc.out.Statics, st)
	return nil
}
