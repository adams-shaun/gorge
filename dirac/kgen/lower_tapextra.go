package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// tapExtraStatic lowers a `Mode$ TapsForMana | Static$ True` trigger to the
// StatTapExtra static when gorge's planner folds it into a tap's production
// (pay.PaymentPlanFoldableTapMana: one literal symbol and amount, no other
// parameter). Anything else keeps refusing: an any-colour or reflected body is
// a deferred source in gorge, planned by hand in the mana window.
func (lc *lowerCtx) tapExtraStatic(t cards.Trigger) error {
	tm, ok := pay.PaymentPlanFoldableTapMana(lc.face, t)
	if !ok {
		return refuse("trigger mode TapsForMana")
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
